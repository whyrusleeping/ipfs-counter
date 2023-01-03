// Stream contains bigquery-specific code for the recorder.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

// Connect sets up a google bigquery client.
func (r *Recorder) Connect(ctx context.Context, dataset, table string) error {
	if dataset == "" || table == "" {
		return errors.New("must specify both dataset and table")
	}

	project := ""
	// Get the bigquery credentials as set.
	if credFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); len(credFile) > 0 {
		f, err := os.ReadFile(credFile)
		if err != nil {
			return err
		}
		type credFile struct {
			ProjectID string `json:"project_id"`
		}
		var creds credFile
		if err := json.Unmarshal(f, &creds); err != nil {
			return err
		}
		project = creds.ProjectID
	} else if proj := os.Getenv("GOOGLE_APPLICATION_PROJECT_ID"); len(proj) > 0 {
		project = proj
	}

	if project == "" {
		return errors.New("could not determine project from environment")
	}

	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return err
	}
	r.Client = client
	r.nodeStream = make(chan *Node, 5)
	r.trialStream = make(chan []*Trial, 5)

	go r.insert(ctx, dataset, table+"_node", table+"_trial")
	return nil
}

// setupBigquery creates tables with schemas if they do not yet exist for the given table prefix
func (r *Recorder) setupBigquery(c context.Context, dataset, table string, create bool) error {
	ds := r.Client.Dataset(dataset)
	nt := ds.Table(table + "_node")
	if _, err := nt.Metadata(c); err != nil {
		if !create {
			return err
		}
		ns, err := bigquery.InferSchema(new(Node))
		if err != nil {
			return err
		}
		err = nt.Create(c, &bigquery.TableMetadata{
			Name:             "IPFS crawler observed Nodes",
			Description:      fmt.Sprintf("Node observations created %s version %s", time.Now(), Version),
			Schema:           ns,
			TimePartitioning: &bigquery.TimePartitioning{Field: "observed"},
		})
		if err != nil {
			return err
		}
	}

	tt := ds.Table(table + "_trial")
	if _, err := tt.Metadata(c); err != nil {
		if !create {
			return err
		}
		ts, err := bigquery.InferSchema(new(TrialSchema))
		if err != nil {
			return err
		}
		err = tt.Create(c, &bigquery.TableMetadata{
			Name:             "IPFS crawler connection attempts",
			Description:      fmt.Sprintf("Trial observations created %s version %s", time.Now(), Version),
			Schema:           ts,
			TimePartitioning: &bigquery.TimePartitioning{Field: "observed"},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Recorder) getMultiAddrs(ctx context.Context, dataset, table string, where time.Duration) ([]string, error) {
	var q *bigquery.Query
	if where == 0 {
		q = r.Client.Query(fmt.Sprintf("SELECT DISTINCT peer_id, multi_address FROM %s.%s", dataset, table))
	} else {
		q = r.Client.Query(fmt.Sprintf("SELECT DISTINCT peer_id, multi_address FROM %s.%s, UNNEST(results) r WHERE r.Success = true AND observed > @ts_value", dataset, table))
		q.Parameters = []bigquery.QueryParameter{
			{
				Name:  "ts_value",
				Value: time.Now().Add(-1 * where),
			},
		}
	}
	job, err := q.Run(ctx)
	if err != nil {
		return nil, err
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return nil, err
	}
	if err := status.Err(); err != nil {
		return nil, err
	}

	iter, err := q.Read(ctx)
	if err != nil {
		return nil, err
	}

	var row TrialSchema
	err = iter.Next(&row)
	if err == iterator.Done {
		r.log.Warnf("No rows matched query: %s", q.Q)
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, iter.TotalRows)
	ma, err := row.MAString()
	if err != nil {
		r.log.Debugf("Failed to re-generate address for peer: %v", err)
	} else {
		out = append(out, ma)
	}

	for {
		err := iter.Next(&row)
		if err == iterator.Done {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		ma, err = row.MAString()
		if err != nil {
			r.log.Debugf("Failed to re-generate address for peer: %v", err)
		} else {
			out = append(out, ma)
		}
	}
}

func (r *Recorder) insert(ctx context.Context, dataset, nodeTable, trialTable string) {
	nodeInserter := r.Client.Dataset(dataset).Table(nodeTable).Inserter()
	trialInserter := r.Client.Dataset(dataset).Table(trialTable).Inserter()
	r.wg.Add(1)
	defer r.wg.Done()

	done := ctx.Done()

	for {
		if r.nodeStream == nil && r.trialStream == nil {
			return
		}

		select {
		case n, ok := <-r.nodeStream:
			if !ok {
				r.nodeStream = nil
				continue
			}
			if err := nodeInserter.Put(ctx, n); err != nil {
				r.log.Warnf("Failed to upload %v: %v", n, err)
			}
		case t, ok := <-r.trialStream:
			if !ok {
				r.trialStream = nil
				continue
			}
			if err := trialInserter.Put(ctx, t); err != nil {
				r.log.Warnf("Failed to upload trial %v: %v", t, err)
			}
		case <-done:
			close(r.nodeStream)
			close(r.trialStream)
			done = nil
		}
	}
}

// Finish closes out remaining uploads for the recorder.
func (r *Recorder) Finish() error {
	if r.trialStream != nil {
		trials := make([]*Trial, 0, 1000)
		i := 0
		r.dials.Range(func(_, val interface{}) bool {
			trials = append(trials, val.(*Trial))
			i++
			if len(trials) > 1000 {
				r.trialStream <- trials
				trials = make([]*Trial, 0, 1000)
			}
			return true
		})
		r.trialStream <- trials
		r.log.Info("Total trials: ", i)
	}
	r.log.Info("Closing context...")
	time.Sleep(time.Second)
	r.cancel()
	r.log.Info("Waiting for all queries to finish...")
	r.wg.Wait()
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

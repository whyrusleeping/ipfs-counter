package main

import (
	"encoding/json"
	"os"
)

// Output writes recorded output to a file at filePath
func Output(filePath string, r *Recorder) error {
	if err := OutputNodes(filePath+".nodes.json", r); err != nil {
		return err
	}
	return OutputTrials(filePath+".trials.json", r)
}

// OutputNodes writes recorded output to a file at filePath
func OutputNodes(filePath string, r *Recorder) error {
	f, err := os.Create(filePath)
	defer f.Close()
	if err != nil {
		return err
	}

	e := json.NewEncoder(f)
	for _, rec := range r.records {
		err := e.Encode(rec)
		if err != nil {
			return err
		}
	}
	return nil
}

// OutputTrials writes recorded output to a file at filePath
func OutputTrials(filePath string, r *Recorder) error {
	f, err := os.Create(filePath)
	defer f.Close()
	if err != nil {
		return err
	}

	e := json.NewEncoder(f)
	for _, rec := range r.dials {
		err := e.Encode(rec)
		if err != nil {
			return err
		}
	}
	return nil
}

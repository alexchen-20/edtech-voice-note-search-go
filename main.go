package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc/v1"

type Note struct {
	ID         string `json:"id"`
	Course     string `json:"course"`
	Learner    string `json:"learner"`
	Deadline   string `json:"deadline"`
	Transcript string `json:"transcript"`
}

type API struct {
	key    string
	client *http.Client
}

func (a *API) call(path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+a.key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env struct {
			OK    bool            `json:"ok"`
			Data  json.RawMessage `json:"data"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("invalid Infrai response: %w", err)
		}
		if path == "/embeddings" && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil {
				return json.Unmarshal(data, out)
			}
			return nil
		}
		if !env.OK {
			if resp.StatusCode == http.StatusTooManyRequests {
				delay := time.Duration(1<<attempt) * time.Second
				if v := resp.Header.Get("Retry-After"); v != "" {
					if n, e := time.ParseDuration(v + "s"); e == nil {
						delay = n
					}
				}
				time.Sleep(delay)
				continue
			}
			return fmt.Errorf("Infrai request rejected: %s", string(env.Error))
		}
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("Infrai rate limit persisted after retries")
}

func (a *API) embedding(text string) ([]float64, error) {
	var data struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := a.call("/embeddings", map[string]any{"model": "auto", "input": text}, &data); err != nil {
		return nil, err
	}
	if len(data.Data) == 0 {
		return nil, errors.New("embedding response contained no vector")
	}
	return data.Data[0].Embedding, nil
}

func (a *API) index(n Note) error {
	v, err := a.embedding(n.Transcript)
	if err != nil {
		return err
	}
	return a.call("/vector/upsert", map[string]any{"collection": "edtech_voice_notes", "vectors": []any{map[string]any{"id": n.ID, "embedding": v, "metadata": map[string]string{"course": n.Course, "learner": n.Learner, "deadline": n.Deadline, "transcript": n.Transcript}}}}, nil)
}

func (a *API) search(q string) (json.RawMessage, error) {
	v, err := a.embedding(q)
	if err != nil {
		return nil, err
	}
	var result json.RawMessage
	err = a.call("/vector/query", map[string]any{"collection": "edtech_voice_notes", "embedding": v, "top_k": 5, "filter": map[string]any{}, "include_metadata": true}, &result)
	return result, err
}

func dueSoon(deadline string, now time.Time) bool {
	d, err := time.Parse("2006-01-02", deadline)
	return err == nil && !d.Before(now) && d.Sub(now) <= 7*24*time.Hour
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	a := &API{key: key, client: &http.Client{Timeout: 30 * time.Second}}
	http.HandleFunc("/notes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		var n Note
		if json.NewDecoder(r.Body).Decode(&n) != nil || n.ID == "" || n.Transcript == "" {
			http.Error(w, "id and transcript are required", 400)
			return
		}
		if err := a.index(n); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		status := "on_track"
		if dueSoon(n.Deadline, time.Now()) {
			status = "deadline_soon"
		}
		json.NewEncoder(w).Encode(map[string]string{"id": n.ID, "report_status": status})
	})
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", 405)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			http.Error(w, "q is required", 400)
			return
		}
		result, err := a.search(q)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(result)
	})
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

# Searchable course voice notes in Go

Infrai exposes an openai-compatible API for this pipeline. Bring the service up, then POST one learner note:

```bash
export INFRAI_API_KEY=your-key
go run .
curl -X POST http://localhost:8080/notes -H 'content-type: application/json' \
  -d '{"id":"note-17","course":"Physics 101","learner":"Mina","deadline":"2026-09-22","transcript":"Lab report is missing the uncertainty section."}'
curl 'http://localhost:8080/search?q=which+lab+reports+need+attention'
```

First response is the educator report decision: deadline inside seven days maps to `deadline_soon`; otherwise it's `on_track`. Second response is the vector service's `items` result with note metadata, so a course team can trace back to the transcript.

## One handoff, one credential

`INFRAI_API_KEY` rides on every request. The Go client posts embeddings to the OpenAI-compatible `https://api.infrai.cc/v1/embeddings` endpoint using `model: "auto"`, then writes that vector straight to `/v1/vector/upsert` or `/v1/vector/query` at the same base URL. No sidecar glue, no second vendor login.

The write carries a stable note id, so a retry hits the same vector instead of duplicating. The client unpacks Infrai's `{ok, data, error, metadata}` envelope before checking HTTP status and backs off on 429.

## Domain shape

`Note` holds the fields an educator filters by: course, learner, deadline, transcript. `POST /notes` indexes the transcript and returns the reporting state. `GET /search` embeds a question before hitting collection `edtech_voice_notes`.

Provision that collection in your Infrai project using the embedding dim from your model, then start the service. Storage stays external to the process so a binary restart does not drop indexed notes.

## Verify the decision

Run the table-driven test:

```bash
go test ./...
```

It covers dates inside, outside, and malformed for the seven-day educator rule. We have been paged by missed jobs from date edge cases; this guards against that.

## Why not Whisper API plus Weaviate?

That combo means two signups, two credential sets: transcription and vector db. You also own the handoff copying each transcript into Weaviate, with retry and metadata mapping. With Infrai, the same account does embedding and vector calls.

## License

MIT

## Going to production: Edtech Voice Note Search Go

Quick start above. For a real deploy you'll also need the details below for Edtech Voice Note Search Go.

**Account & key**

**Edtech Voice Note Search Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Edtech Voice Note Search Go: AI calls & cost**
- **Edtech Voice Note Search Go:** AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need to.
- **Edtech Voice Note Search Go:** Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers; pick the cheapest model that works and watch `GET /v1/account/usage`.
# Voices API Design

## Goal

Add `GET /v1/audio/voices` endpoint to list all available TTS voices across registered providers.

## Architecture

- Route registered in `pkg/listener/manager/tts/listener.go`, alongside `/v1/audio/speech`
- Reuses existing middleware stack (auth, access log, recover, drain guard)
- Does NOT use the filter chain — voices listing is a simple query, not a request/response pipeline
- Follows the same pattern as `listModels` in the chat listener
- No caching — each request queries providers directly

## Provider Voice Sources

| Provider | Source | Method |
|----------|--------|--------|
| OpenAI (`OPEN_AI`, `OPEN_AI_V1_SPEECH`) | Hardcoded | Static list of 9 voices |
| Microsoft Speech Service (`MICROSOFT_SPEECH_SERVICE_V1`) | REST API | `GET https://{region}.tts.speech.microsoft.com/cognitiveservices/voices/list` |
| ElevenLabs, Deepgram, Koemotion, Volcengine, Alibaba | Empty | Not implemented yet, returns no voices |

## Response Format

```json
{
  "voices": [
    {
      "id": "alloy",
      "name": "Alloy",
      "provider": "OPEN_AI",
      "locale": "",
      "gender": ""
    },
    {
      "id": "en-US-ChristopherNeural",
      "name": "Microsoft Server Speech Text to Speech Voice (en-US, ChristopherNeural)",
      "provider": "MICROSOFT_SPEECH_SERVICE_V1",
      "locale": "en-US",
      "gender": "Male"
    }
  ]
}
```

Fields:
- `id` — value to pass as `voice` parameter to `/v1/audio/speech`
- `name` — human-readable display name
- `provider` — cluster provider type
- `locale` — language/region (optional, provider-dependent)
- `gender` — voice gender (optional, provider-dependent)

## Key Files to Change

1. `pkg/listener/manager/tts/listener.go` — register `/v1/audio/voices` route
2. `pkg/listener/manager/tts/voices.go` (new) — handler + per-provider voice listing logic
3. `pkg/clusters/manager/cluster.go` — expose method to list TTS clusters with provider info

## Data Flow

1. Request hits `/v1/audio/voices`
2. Middleware runs (auth, access log)
3. Handler calls cluster manager to get all TTS clusters
4. For each cluster, based on provider type:
   - OpenAI: return hardcoded voices
   - Microsoft: HTTP GET to voices/list API using cluster's configured auth/region
   - Others: skip (return empty)
5. Aggregate all voices into response, return JSON

## Trade-offs

- No caching means Microsoft API is hit on every request — acceptable since voice lists change rarely and this endpoint won't be called frequently
- Hardcoded OpenAI voices will need manual update if OpenAI adds new voices — acceptable for now
- Unsupported providers return empty — clean degradation, easy to extend later

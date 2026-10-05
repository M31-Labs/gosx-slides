---
title: A request worth explaining
theme: aurora
story: story.yaml
offline-required: true
---

```yaml
id: request
cues: overview, accepted, completed
```

# Explain the request once

<Scene3D Src="request.sir" />

```go
request := accept()
enqueue(request)
persist(request)
```

<p data-story-id="completion">The worker has persisted the request.</p>

<!-- Advance through the named cues. Focus, camera, code, visibility and captions all come from story.yaml. -->

---
title: Slidev migration
theme: default
layout: cover
---

# Start with a question

![Local brand](/logo.svg)

<!-- PRIVATE SLIDEV NOTES: Keep this out of audience exports. -->

---
layout: center
---

# Show the mechanism

<v-clicks>

- Read a source
- Validate a candidate
- Publish a result

</v-clicks>

```ts {1-2|3}
const source = read()
const candidate = compile(source)
publish(candidate)
```

<CustomWidget :value="answer" />

{{ userCode() }}

<!-- PRIVATE SECOND NOTES -->

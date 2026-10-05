---
title: Portable Office content
theme: aurora
aspect-ratio: 4:3
transition: none
---

# An editable table

| Capability | Value |
|---|---|
| Native tables | 42 |
| Editable chart data | 7 |
| Pixel fallback | 11 |

<!-- Simple rectangular tables become native PowerPoint tables with editable cells. Merged cells, rich cell formatting and clipped tables retain captured pixels. -->

---

# Compare the values

```sirena diagram=bar
service authoring { label: "Authoring" value: 45 }
service presenting { label: "Presenting" value: 35 }
service sharing { label: "Sharing" value: 20 }
```

<!-- Editable export translates the displayed Sirena values into a native horizontal bar chart and an embedded workbook. PowerPoint controls chart layout and typography. -->

---

# Share the composition

```sirena diagram=pie
service authoring { label: "Authoring" value: 45 }
service presenting { label: "Presenting" value: 35 }
service sharing { label: "Sharing" value: 20 }
```

<!-- The captured export preserves the original pixels. Editable export preserves the simple chart categories and displayed values. -->

package main

// Citation is a small static evidence chip: a quantitative claim on a slide
// points at the artifact that produced it (a PR, a floor file, a bench run).
// The conference validation profile requires one alongside any evidence
// island. Props bind by exact attribute name; the
// deck-side analysis (slideCitations) reads lowercase href/label from the
// tag, so this island uses lowercase prop names throughout.
//
//gosx:island
func Citation(props any) Node {
	return <a href={props.href} target="_blank" class="citation" style="display:inline-flex;align-items:center;gap:0.5rem;font-family:var(--font-mono);font-size:0.85rem;color:var(--fg-muted);text-decoration:none;border:1px solid var(--line);border-radius:999px;padding:0.35rem 0.9rem;background:var(--surface)">
		<span style="color:var(--accent)">◈</span>
		<span>{props.label}</span>
	</a>
}

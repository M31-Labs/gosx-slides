(() => {
  const slide = document.querySelector(".slide.deck-active"),
    objects = [],
    undo = [];
  let glyphs = 0;
  function visible(el) {
    const s = getComputedStyle(el),
      r = el.getBoundingClientRect();
    return (
      s.display !== "none" &&
      s.visibility === "visible" &&
      Number(s.opacity) === 1 &&
      (r.width > 0 || r.height > 0) &&
      r.left >= -1 &&
      r.top >= -1 &&
      r.right <= innerWidth + 1 &&
      r.bottom <= innerHeight + 1
    );
  }
  function color(value) {
    if (value === "none" || value === "transparent") return "";
    const match =
      /^rgba?\((\d+)[, ]+\s*(\d+)[, ]+\s*(\d+)(?:[, /]+\s*([\d.]+))?\)$/.exec(
        value,
      );
    if (!match || (match[4] != null && Number(match[4]) !== 1)) return "";
    return match
      .slice(1, 4)
      .map((v) => Number(v).toString(16).padStart(2, "0"))
      .join("");
  }
  function safe(el, objectRect = el.getBoundingClientRect()) {
    for (let p = el; p; p = p.parentElement) {
      const s = getComputedStyle(p);
      if (
        Number(s.opacity) !== 1 ||
        s.filter !== "none" ||
        s.clipPath !== "none" ||
        (s.maskImage && s.maskImage !== "none") ||
        (s.clip && s.clip !== "auto") ||
        s.contentVisibility === "hidden"
      )
        return false;
      if (s.transform !== "none") {
        const m = new DOMMatrix(s.transform);
        if (
          Math.abs(m.b) > 1e-6 ||
          Math.abs(m.c) > 1e-6 ||
          m.a <= 0 ||
          m.d <= 0
        )
          return false;
      }
      const clipsX = s.overflowX !== "visible",
        clipsY = s.overflowY !== "visible";
      if (clipsX || clipsY) {
        // Curved clips cannot be represented by these native objects. Keep
        // their captured pixels; rectangular clips are safe only if contained.
        const radii = [
          s.borderTopLeftRadius,
          s.borderTopRightRadius,
          s.borderBottomLeftRadius,
          s.borderBottomRightRadius,
        ];
        const box = p.getBoundingClientRect(),
          sx = box.width / (p.offsetWidth || p.clientWidth || box.width || 1),
          sy =
            box.height / (p.offsetHeight || p.clientHeight || box.height || 1),
          left = box.left + p.clientLeft * sx,
          top = box.top + p.clientTop * sy,
          right = left + p.clientWidth * sx,
          bottom = top + p.clientHeight * sy;
        // The central inset stays inside every rounded corner. Keep edge
        // content captured while ordinary inset diagram shapes stay editable.
        let insetX = 0,
          insetY = 0;
        for (const radius of radii) {
          if (!radius) continue;
          const parts = radius.trim().split(/\s+/);
          if (parts.some((part) => !/^\d*\.?\d+(?:px|%)$/.test(part)))
            return false;
          const rx = parts[0],
            ry = parts[1] || rx;
          insetX = Math.max(
            insetX,
            parseFloat(rx) * (rx.endsWith("%") ? (right - left) / 100 : sx),
          );
          insetY = Math.max(
            insetY,
            parseFloat(ry) * (ry.endsWith("%") ? (bottom - top) / 100 : sy),
          );
        }

        if (
          (clipsX &&
            (objectRect.left < left + insetX - 0.01 ||
              objectRect.right > right - insetX + 0.01)) ||
          (clipsY &&
            (objectRect.top < top + insetY - 0.01 ||
              objectRect.bottom > bottom - insetY + 0.01))
        )
          return false;
      }
    }
    return true;
  }
  const walker = document.createTreeWalker(slide, NodeFilter.SHOW_TEXT),
    texts = [];
  while (walker.nextNode()) texts.push(walker.currentNode);
  for (const node of texts) {
    const el = node.parentElement;
    if (
      !node.textContent.trim() ||
      el.closest(
        'svg,canvas,.slide-graphic,.deck-graphics-background,script,style,aside,button,input,textarea,select,dialog,[aria-hidden="true"],.speaker-note',
      ) ||
      !visible(el) ||
      !safe(el) ||
      glyphs + node.length > 20000
    )
      continue;
    const s = getComputedStyle(el);
    if (!color(s.color)) continue;
    const range = document.createRange();
    range.selectNodeContents(node);
    const rects = Array.from(range.getClientRects());
    if (
      !rects.length ||
      rects.some(
        (rect) =>
          !safe(el, rect) ||
          rect.left < 0 ||
          rect.top < 0 ||
          rect.right > innerWidth ||
          rect.bottom > innerHeight,
      )
    )
      continue;
    const rows = [];
    if (rects.length === 1) rows.push({ text: node.textContent, r: rects[0] });
    else {
      for (let i = 0; i < node.length; i++) {
        range.setStart(node, i);
        range.setEnd(node, i + 1);
        const r = range.getBoundingClientRect();
        if (r.width === 0) continue;
        let row = rows.at(-1);
        if (!row || Math.abs(row.r.top - r.top) > 1) {
          row = {
            text: "",
            r: { left: r.left, top: r.top, width: 0, height: r.height },
          };
          rows.push(row);
        }
        row.text += node.textContent[i];
        row.r.width = Math.max(row.r.width, r.right - row.r.left);
      }
    }
    glyphs += node.length;
    const wrapper = document.createElement("span");
    wrapper.style.visibility = "hidden";
    node.replaceWith(wrapper);
    wrapper.appendChild(node);
    undo.push(() => wrapper.replaceWith(node));
    for (const { text, r } of rows) {
      if (!text.trim()) continue;
      objects.push({
        kind: "text",
        text,
        x: r.left,
        y: r.top,
        width: r.width,
        height: r.height,
        fontSize: parseFloat(s.fontSize),
        fontFamily: s.fontFamily.split(",")[0].replace(/^['"]|['"]$/g, ""),
        color: color(s.color),
        bold: Number(s.fontWeight) >= 600,
        italic: s.fontStyle === "italic",
      });
    }
  }
  const svgShapes = slide.querySelectorAll(
    "svg rect,svg circle,svg ellipse,svg line,svg path,svg polyline,svg polygon,svg g.label[aria-label]",
  );
  for (const el of svgShapes) {
    if (
      el.closest("defs,marker,clipPath,mask,[hidden]") ||
      el.closest('[aria-hidden="true"]') ||
      (el.closest("g.label") !== el && el.closest("g.label")) ||
      !visible(el) ||
      !safe(el)
    )
      continue;
    const s = getComputedStyle(el),
      r = el.getBoundingClientRect(),
      m = el.getScreenCTM();
    if (
      !m ||
      Math.abs(m.b) > 1e-6 ||
      Math.abs(m.c) > 1e-6 ||
      m.a <= 0 ||
      m.d <= 0
    )
      continue;
    let kind = el.localName;
    if (kind === "g") {
      objects.push({
        kind: "text",
        text: el.getAttribute("aria-label"),
        x: r.left,
        y: r.top,
        width: r.width,
        height: r.height,
        fontSize: 14 * m.a,
        fontFamily: "Arial",
        color: color(s.fill) || "000000",
      });
    } else {
      const fill = color(s.fill),
        stroke = color(s.stroke);
      if ((s.fill !== "none" && !fill) || (s.stroke !== "none" && !stroke))
        continue;
      let path = [];
      if (kind === "path") {
        if (s.markerStart !== "none" || s.markerEnd !== "none") continue;
        const d = el.getAttribute("d") || "",
          tokens =
            d.match(
              /[MLCQZmlcqz]|[-+]?(?:\d*\.\d+|\d+\.?\d*)(?:e[-+]?\d+)?/gi,
            ) || [];
        if (d.replace(/[MLCQZ\s,\d.eE+\-]/g, "") || tokens.length > 1024)
          continue;
        let i = 0,
          cmd;
        while (i < tokens.length) {
          if (/^[MLCQZ]$/.test(tokens[i])) cmd = tokens[i++];
          else if (!cmd) {
            path = [];
            break;
          }
          const count = { M: 2, L: 2, C: 6, Q: 4, Z: 0 }[cmd];
          if (count == null || i + count > tokens.length) {
            path = [];
            break;
          }
          const points = [];
          for (let k = 0; k < count; k += 2) {
            const x = Number(tokens[i++]),
              y = Number(tokens[i++]);
            if (!Number.isFinite(x) || !Number.isFinite(y)) {
              path = [];
              i = tokens.length;
              break;
            }
            const p = new DOMPoint(x, y).matrixTransform(m);
            points.push(p.x - r.left, p.y - r.top);
          }
          path.push({ command: cmd, points });
          if (cmd === "M") cmd = "L";
          if (cmd === "Z") cmd = null;
        }
        if (!path.length) continue;
        kind = "path";
      } else if (kind === "polyline" || kind === "polygon") {
        const list = Array.from(el.points);
        if (list.length > 256 || list.length < 2) continue;
        path = list.map((p, i) => {
          const v = new DOMPoint(p.x, p.y).matrixTransform(m);
          return {
            command: i ? "L" : "M",
            points: [v.x - r.left, v.y - r.top],
          };
        });
        if (kind === "polygon") path.push({ command: "Z", points: [] });
        kind = "path";
      } else if (kind === "rect")
        kind = Number(el.getAttribute("rx")) > 0 ? "roundRect" : "rect";
      else if (kind === "circle") kind = "ellipse";
      else if (kind !== "ellipse" && kind !== "line") continue;
      const flipV =
        kind === "line" &&
        (el.x2.baseVal.value - el.x1.baseVal.value) *
          (el.y2.baseVal.value - el.y1.baseVal.value) <
          0;
      objects.push({
        kind,
        x: r.left,
        y: r.top,
        width: r.width,
        height: r.height,
        fill,
        stroke,
        fillAlpha: Number(s.fillOpacity),
        strokeAlpha: Number(s.strokeOpacity),
        strokeWidth: parseFloat(s.strokeWidth) * m.a,
        path,
        flipV,
      });
    }
    const visibility = el.style.visibility;
    el.style.visibility = "hidden";
    undo.push(() => (el.style.visibility = visibility));
  }
  window.__slidesPPTXRestore = () => {
    undo.reverse().forEach((fn) => fn());
    delete window.__slidesPPTXRestore;
  };
  return objects;
})();

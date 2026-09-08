let cards = [];
let currentId = null;
let photoIndex = 0;
const MAX_PHOTOS = 4;

const el = (id) => document.getElementById(id);

async function api(path, opts = {}) {
  const res = await fetch(path, opts);
  if (!res.ok) throw new Error(await res.text() || res.statusText);
  if (res.status === 204) return null;
  return res.json();
}

function setStatus(msg, ok = true) {
  const s = el("status");
  s.textContent = msg || "";
  s.style.color = ok ? "#22c55e" : "#fca5a5";
}

function cardPhotos(c) {
  if (!c) return [];
  if (Array.isArray(c.photos) && c.photos.length) return c.photos;
  if (c.photoFile) return [c.photoFile];
  return [];
}

function currentCard() {
  return cards.find((x) => x.id === currentId) || null;
}

function renderList() {
  const ul = el("cardList");
  ul.innerHTML = "";
  cards
    .slice()
    .sort((a, b) => String(b.updatedAt).localeCompare(String(a.updatedAt)))
    .forEach((c) => {
      const n = cardPhotos(c).length;
      const li = document.createElement("li");
      const btn = document.createElement("button");
      btn.type = "button";
      if (c.id === currentId) btn.classList.add("active");
      btn.innerHTML = `<span class="name">${escapeHtml(c.name || "Без названия")}</span><span class="meta">${n ? `📷 ${n}/4` : "без фото"}</span>`;
      btn.onclick = () => selectCard(c.id);
      li.appendChild(btn);
      ul.appendChild(li);
    });
}

function escapeHtml(s) {
  return String(s)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function showEditor(show) {
  el("emptyState").classList.toggle("hidden", show);
  el("editor").classList.toggle("hidden", !show);
}

function fillEditor(c) {
  el("fieldName").value = c?.name || "";
  el("fieldDesc").value = c?.description || "";
  const photos = cardPhotos(c);
  if (photoIndex >= photos.length) photoIndex = Math.max(0, photos.length - 1);
  renderGallery(photos);
}

function renderGallery(photos) {
  const img = el("photoPreview");
  const ph = el("photoPlaceholder");
  const counter = el("photoCounter");
  const prev = el("btnPrev");
  const next = el("btnNext");
  const addLabel = el("photoAddLabel");
  const removeBtn = el("btnRemovePhoto");

  if (!photos.length) {
    img.removeAttribute("src");
    img.classList.add("hidden");
    ph.classList.remove("hidden");
    counter.textContent = "0 / 4";
    prev.classList.remove("visible");
    next.classList.remove("visible");
    removeBtn.classList.add("hidden");
    addLabel.classList.remove("hidden");
    return;
  }

  const file = photos[photoIndex];
  img.src = `/api/photos/${encodeURIComponent(file)}?t=${Date.now()}`;
  img.classList.remove("hidden");
  ph.classList.add("hidden");
  counter.textContent = `${photoIndex + 1} / ${photos.length} · макс. 4`;
  const multi = photos.length > 1;
  prev.classList.toggle("visible", multi);
  next.classList.toggle("visible", multi);
  removeBtn.classList.remove("hidden");
  addLabel.classList.toggle("hidden", photos.length >= MAX_PHOTOS);
}

async function refresh() {
  cards = await api("/api/cards");
  renderList();
  if (currentId) {
    const c = cards.find((x) => x.id === currentId);
    if (c) {
      showEditor(true);
      fillEditor(c);
    } else {
      currentId = null;
      showEditor(false);
    }
  }
}

async function selectCard(id) {
  currentId = id;
  photoIndex = 0;
  const c = await api(`/api/cards/${id}`);
  const idx = cards.findIndex((x) => x.id === id);
  if (idx >= 0) cards[idx] = c;
  else cards.push(c);
  showEditor(true);
  fillEditor(c);
  renderList();
  setStatus("");
}

el("btnPrev").onclick = () => {
  const photos = cardPhotos(currentCard());
  if (photos.length < 2) return;
  photoIndex = (photoIndex - 1 + photos.length) % photos.length;
  renderGallery(photos);
};

el("btnNext").onclick = () => {
  const photos = cardPhotos(currentCard());
  if (photos.length < 2) return;
  photoIndex = (photoIndex + 1) % photos.length;
  renderGallery(photos);
};

document.addEventListener("keydown", (e) => {
  if (!currentId || el("editor").classList.contains("hidden")) return;
  if (e.target && (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA")) return;
  if (e.key === "ArrowLeft") el("btnPrev").click();
  if (e.key === "ArrowRight") el("btnNext").click();
});

el("btnNew").onclick = async () => {
  const c = await api("/api/cards", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: "Новая карточка", description: "" }),
  });
  currentId = c.id;
  photoIndex = 0;
  await refresh();
  setStatus("Создано");
};

el("btnSave").onclick = async () => {
  if (!currentId) return;
  try {
    await api(`/api/cards/${currentId}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: el("fieldName").value,
        description: el("fieldDesc").value,
      }),
    });
    await refresh();
    setStatus("Сохранено");
  } catch (e) {
    setStatus(String(e.message || e), false);
  }
};

el("btnDelete").onclick = async () => {
  if (!currentId) return;
  if (!confirm("Удалить эту карточку?")) return;
  await api(`/api/cards/${currentId}`, { method: "DELETE" });
  currentId = null;
  showEditor(false);
  await refresh();
  setStatus("Удалено");
};

el("btnRemovePhoto").onclick = async () => {
  if (!currentId) return;
  const photos = cardPhotos(currentCard());
  if (!photos.length) return;
  try {
    const c = await api(`/api/cards/${currentId}/photo/${photoIndex}`, { method: "DELETE" });
    const idx = cards.findIndex((x) => x.id === currentId);
    if (idx >= 0) cards[idx] = c;
    if (photoIndex >= cardPhotos(c).length) photoIndex = Math.max(0, cardPhotos(c).length - 1);
    fillEditor(c);
    renderList();
    setStatus("Фото убрано");
  } catch (e) {
    setStatus(String(e.message || e), false);
  }
};

el("photoInput").onchange = async (ev) => {
  const file = ev.target.files?.[0];
  if (!file || !currentId) return;
  const photos = cardPhotos(currentCard());
  if (photos.length >= MAX_PHOTOS) {
    setStatus("Уже 4 фото — максимум", false);
    ev.target.value = "";
    return;
  }
  const fd = new FormData();
  fd.append("photo", file);
  try {
    const c = await api(`/api/cards/${currentId}/photo`, { method: "POST", body: fd });
    const idx = cards.findIndex((x) => x.id === currentId);
    if (idx >= 0) cards[idx] = c;
    photoIndex = cardPhotos(c).length - 1;
    fillEditor(c);
    renderList();
    setStatus(`Фото ${cardPhotos(c).length}/4`);
  } catch (e) {
    setStatus(String(e.message || e), false);
  }
  ev.target.value = "";
};

refresh().catch((e) => setStatus(String(e), false));

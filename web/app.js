let cards = [];
let currentId = null;

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

function renderList() {
  const ul = el("cardList");
  ul.innerHTML = "";
  cards
    .slice()
    .sort((a, b) => String(b.updatedAt).localeCompare(String(a.updatedAt)))
    .forEach((c) => {
      const li = document.createElement("li");
      const btn = document.createElement("button");
      btn.type = "button";
      if (c.id === currentId) btn.classList.add("active");
      btn.innerHTML = `<span class="name">${escapeHtml(c.name || "Без названия")}</span><span class="meta">${c.photoFile ? "📷 есть фото" : "без фото"}</span>`;
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
  const img = el("photoPreview");
  const ph = el("photoPlaceholder");
  if (c?.photoFile) {
    img.src = `/api/photos/${encodeURIComponent(c.photoFile)}?t=${Date.now()}`;
    img.classList.remove("hidden");
    ph.classList.add("hidden");
  } else {
    img.removeAttribute("src");
    img.classList.add("hidden");
    ph.classList.remove("hidden");
  }
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
  const c = await api(`/api/cards/${id}`);
  showEditor(true);
  fillEditor(c);
  renderList();
  setStatus("");
}

el("btnNew").onclick = async () => {
  const c = await api("/api/cards", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: "Новая карточка", description: "" }),
  });
  currentId = c.id;
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

el("photoInput").onchange = async (ev) => {
  const file = ev.target.files?.[0];
  if (!file || !currentId) return;
  const fd = new FormData();
  fd.append("photo", file);
  try {
    await api(`/api/cards/${currentId}/photo`, { method: "POST", body: fd });
    await refresh();
    setStatus("Фото загружено");
  } catch (e) {
    setStatus(String(e.message || e), false);
  }
  ev.target.value = "";
};

refresh().catch((e) => setStatus(String(e), false));

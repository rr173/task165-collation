const $ = (selector) => document.querySelector(selector);
const projectSelect = $("#project-select");
const notice = $("#notice");

async function api(path, options) {
  const response = await fetch(path, { headers: { "Content-Type": "application/json" }, ...options });
  const payload = await response.json();
  if (!response.ok || !payload.ok) throw new Error(payload.error || `请求失败（${response.status}）`);
  return payload.data;
}

function setNotice(text, error = false) {
  notice.textContent = text;
  notice.style.color = error ? "#ffd2c7" : "#d9ebdf";
}

function empty(target, text) { target.replaceChildren(Object.assign(document.createElement("p"), { className: "empty", textContent: text })); }

async function refreshProjects() {
  const projects = await api("/api/projects");
  const selected = projectSelect.value;
  projectSelect.replaceChildren(new Option("选择工程", ""));
  projects.forEach((project) => projectSelect.add(new Option(`${project.name} · ${project.status}`, project.id)));
  projectSelect.value = selected;
  setNotice(`已加载 ${projects.length} 个校勘工程。`);
}

async function passagesFor(witness) {
  return api(`/api/witnesses/${encodeURIComponent(witness.id)}/passages`);
}

function renderPassages(target, witness, passages) {
  target.replaceChildren();
  const heading = document.createElement("p");
  heading.className = "variant-meta";
  heading.textContent = `${witness.code} · ${witness.title}`;
  target.append(heading);
  if (!passages.length) { target.append(Object.assign(document.createElement("p"), { className: "empty", textContent: "该见证本还没有导入段落。" })); return; }
  const template = $("#passage-template");
  passages.forEach((passage) => {
    const node = template.content.firstElementChild.cloneNode(true);
    node.querySelector("header").textContent = `段落 ${passage.ordinal}`;
    node.querySelector("p").textContent = passage.text;
    target.append(node);
  });
}

function renderVariants(variants) {
  const target = $("#variant-cards");
  target.replaceChildren();
  if (!variants.length) { empty(target, "当前工程尚未生成异文位。确认锚点后可执行区间对齐。 "); return; }
  variants.forEach((variant) => {
    const card = document.createElement("article");
    card.className = "variant";
    const title = document.createElement("h3");
    title.textContent = `${variant.diff_type} 异文位 · ${variant.status}`;
    const meta = document.createElement("p");
    meta.className = "variant-meta";
    meta.textContent = `底本范围 ${variant.base_start_char}–${variant.base_end_char}；认领人：${variant.claimed_by || "未认领"}`;
    card.append(title, meta);
    target.append(card);
  });
}

// loadSnapshotDetail fetches the project's most recent snapshot and renders the
// frozen original-text evidence plus whether the current body still verifies.
async function loadSnapshotDetail(projectID) {
  const target = $("#snapshot-detail");
  try {
    const snapshots = await api(`/api/projects/${encodeURIComponent(projectID)}/snapshots`);
    if (!snapshots || !snapshots.length) { empty(target, "尚无已构建的定本快照。构建并发布快照后，此处呈现原文冻结依据。"); return; }
    const snapshot = snapshots[0];
    const view = await api(`/api/snapshots/${encodeURIComponent(snapshot.id)}`);

    const heading = document.createElement("p");
    heading.className = "variant-meta";
    heading.textContent = `${snapshot.title || "定本快照"} · 第 ${snapshot.round_no} 轮 · ${snapshot.status}`;
    target.replaceChildren(heading);

    const verdict = document.createElement("p");
    verdict.className = "integrity-verdict";
    if (!view.integrity_hash) {
      verdict.textContent = "正文不可验证：本快照未冻结完整性基准。";
      verdict.classList.add("integrity-bad");
    } else if (view.integrity_ok) {
      verdict.textContent = "正文仍可验证 ✓（冻结基准与当前重算一致）";
      verdict.classList.add("integrity-good");
    } else {
      verdict.textContent = "正文不可验证 ✗（冻结基准 ≠ 当前重算，原文证据已被改动）";
      verdict.classList.add("integrity-bad");
    }
    target.append(verdict);

    const hashesHeading = document.createElement("p");
    hashesHeading.className = "variant-meta";
    hashesHeading.textContent = `原文冻结依据：${view.passage_count} 个底本段落哈希；锚点 ${view.anchor_count} 项，决定 ${view.decision_count} 项。`;
    target.append(hashesHeading);

    const list = document.createElement("ul");
    list.className = "hash-list";
    Object.entries(view.passage_hashes || {}).forEach(([passageID, hash]) => {
      const item = document.createElement("li");
      const id = document.createElement("span");
      id.className = "hash-id";
      id.textContent = passageID;
      const h = document.createElement("code");
      h.textContent = hash;
      item.append(id, h);
      list.append(item);
    });
    target.append(list);

    const bodyHeading = document.createElement("p");
    bodyHeading.className = "variant-meta";
    bodyHeading.textContent = "冻结正文";
    target.append(bodyHeading);
    const body = document.createElement("p");
    body.className = "snapshot-body";
    body.textContent = view.snapshot.body || "（空）";
    target.append(body);

    const hashes = document.createElement("p");
    hashes.className = "variant-meta hash-baseline";
    hashes.textContent = `冻结基准 ${view.integrity_hash} · 当前重算 ${view.recomputed_hash}`;
    target.append(hashes);
  } catch (error) { empty(target, `快照详情加载失败：${error.message}`); }
}

async function loadProject() {
  const id = projectSelect.value;
  if (!id) { empty($("#base-passages"), "选择工程后加载底本。"); empty($("#witness-passages"), "选择工程后加载见证本。"); empty($("#variant-cards"), "尚未加载异文位。"); empty($("#definitive-preview"), "尚未加载定本预览。"); empty($("#snapshot-detail"), "尚无已构建的定本快照。"); return; }
  setNotice("正在加载工程中的文本、异文证据和定本预览…");
  try {
    const witnesses = await api(`/api/projects/${encodeURIComponent(id)}/witnesses`);
    const base = witnesses.find((witness) => witness.is_base);
    const alternate = witnesses.find((witness) => !witness.is_base);
    if (base) renderPassages($("#base-passages"), base, await passagesFor(base)); else empty($("#base-passages"), "该工程尚未设定底本。");
    if (alternate) renderPassages($("#witness-passages"), alternate, await passagesFor(alternate)); else empty($("#witness-passages"), "该工程尚未添加见证本。");
    renderVariants(await api(`/api/projects/${encodeURIComponent(id)}/variants`));
    const preview = $("#definitive-preview");
    try {
      const body = await api(`/api/projects/${encodeURIComponent(id)}/snapshot/preview`);
      preview.replaceChildren(Object.assign(document.createElement("p"), { textContent: body || "候选定本当前为空。" }));
    } catch (error) { empty(preview, `尚不能预览定本：${error.message}`); }
    loadSnapshotDetail(id).catch((error) => setNotice(error.message, true));
    setNotice("工程数据已从同一 JSON API 加载。");
  } catch (error) { setNotice(error.message, true); }
}

$("#refresh-projects").addEventListener("click", () => refreshProjects().catch((error) => setNotice(error.message, true)));
projectSelect.addEventListener("change", loadProject);
$("#create-project-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const name = $("#project-name").value.trim();
  if (!name) return;
  try {
    const project = await api("/api/projects", { method: "POST", body: JSON.stringify({ name, description: "由浏览器校勘工作台创建" }) });
    $("#project-name").value = "";
    await refreshProjects();
    projectSelect.value = project.id;
    await loadProject();
  } catch (error) { setNotice(error.message, true); }
});

refreshProjects().catch((error) => setNotice(error.message, true));

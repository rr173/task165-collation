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

async function loadProject() {
  const id = projectSelect.value;
  if (!id) { empty($("#base-passages"), "选择工程后加载底本。"); empty($("#witness-passages"), "选择工程后加载见证本。"); empty($("#variant-cards"), "尚未加载异文位。"); empty($("#definitive-preview"), "尚未加载定本预览。"); return; }
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

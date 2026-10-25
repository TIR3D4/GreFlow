const $ = (s) => document.querySelector(s);
const esc = (v) =>
  String(v ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
const icons = {
  overview:
    '<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>',
  servers:
    '<rect x="3" y="3" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2"/><path d="M7 6h1M7 17h1"/>',
  tunnels: '<path d="M4 7h16l-4-4M20 17H4l4 4"/>',
  ports: '<path d="M8 3v7H3M16 3v7h5M12 10v11M7 17h10"/>',
  operations: '<circle cx="12" cy="12" r="9"/><path d="m10 8 6 4-6 4z"/>',
  schedules: '<circle cx="12" cy="12" r="9"/><path d="M12 6v6l4 2"/>',
  settings:
    '<path d="M4 6h16M4 12h16M4 18h16"/><circle cx="8" cy="6" r="2"/><circle cx="16" cy="12" r="2"/><circle cx="8" cy="18" r="2"/>',
  menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
  moon: '<path d="M21 13a9 9 0 0 1-10-10A9 9 0 1 0 21 13Z"/>',
  close: '<path d="m6 6 12 12M18 6 6 18"/>',
  plus: '<path d="M12 4v16M4 12h16"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  warning: '<path d="m12 3 10 18H2zM12 9v5M12 17v1"/>',
};
const icon = (n) =>
  `<svg viewBox="0 0 24 24" aria-hidden="true">${icons[n] || icons.tunnels}</svg>`;
const nav = [
  ["overview", "نمای کلی"],
  ["servers", "سرورها"],
  ["tunnels", "تانل‌ها"],
  ["ports", "مسیر پورت‌ها"],
  ["operations", "عملیات"],
  ["schedules", "زمان‌بندی"],
  ["settings", "تنظیمات و امنیت"],
];
const labels = {
  ok: "تأیید شد",
  failed: "ناموفق",
  unknown: "نامشخص",
  unsupported: "نیازمند تست برنامه",
  stale: "نیاز به بررسی",
  healthy: "در دسترس",
  degraded: "اختلال پینگ",
  down: "نیازمند بررسی",
  queued: "در صف",
  running: "در حال اجرا",
  succeeded: "انجام شد",
  uncertain: "نتیجه نامشخص",
  rolled_back: "برگشت انجام شد",
  rollback_failed: "برگشت ناموفق",
};
const btn = (label, action, id = "", primary = false) =>
  `<button type="button" class="btn ${primary ? "primary" : ""}" data-action="${action}" data-id="${esc(id)}">${label}</button>`;
const badge = (s, label) =>
  `<span class="chip ${["ok", "healthy", "succeeded"].includes(s) ? "green" : ["failed", "down", "rollback_failed"].includes(s) ? "red" : ["unknown", "stale", "unsupported"].includes(s) ? "neutral" : "amber"}">${icon(["ok", "healthy", "succeeded"].includes(s) ? "check" : ["unknown", "stale", "unsupported"].includes(s) ? "schedules" : "warning")}${esc(label || labels[s] || s)}</span>`;
const time = (v) =>
  v
    ? new Date(v).toLocaleString("fa-IR", {
        timeZone: localStorage.getItem("greflow-timezone") || "Asia/Tehran",
      })
    : "هنوز دریافت نشده";
const heading = (title, desc, action = "") =>
  `<div class="page-heading"><div><h1>${title}</h1><p>${desc}</p></div>${action}</div>`;
const empty = (title, desc, action = "") =>
  `<div class="empty"><div class="empty-icon">${icon("tunnels")}</div><h2>${title}</h2><p>${desc}</p>${action}</div>`;
const field = (name, label, value = "", type = "text", extra = "") =>
  `<div class="field"><label for="f-${name}">${label}</label><div class="input-wrap"><input id="f-${name}" name="${name}" type="${type}" value="${esc(value)}" ${extra}>${type === "password" ? `<button type="button" class="password-toggle" data-action="reveal-password" data-id="f-${name}" aria-label="نمایش ${esc(label)}" aria-pressed="false">نمایش</button>` : ""}</div></div>`;
const select = (name, label, options, value = "") =>
  `<div class="field"><label for="f-${name}">${label}</label><select id="f-${name}" name="${name}">${options.map(([v, t]) => `<option value="${esc(v)}" ${v === value ? "selected" : ""}>${esc(t)}</option>`).join("")}</select></div>`;
let user = null,
  hosts = [],
  operations = [],
  schedules = [],
  capabilities = {},
  stream,
  returnFocus,
  refreshing = false;
const uid = () => crypto.randomUUID().replaceAll("-", "");
const params = () => new URLSearchParams(location.hash.split("?")[1] || "");
const page = () => location.hash.slice(1).split("?")[0] || "overview";
const selected = () => params().get("host") || "";
function routeParam(k, v) {
  const p = params();
  v ? p.set(k, v) : p.delete(k);
  history.replaceState(null, "", `#${page()}?${p}`);
}
async function api(path, body, idem = uid()) {
  const r = await fetch("/api/v1/" + path, {
    method: body === undefined ? "GET" : "POST",
    headers:
      body === undefined
        ? {}
        : {
            "Content-Type": "application/json",
            "X-CSRF-Token": user?.csrf || "",
            "Idempotency-Key": idem,
          },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const v = await r.json();
  if (!r.ok) {
    if (r.status === 401 && user) {
      user = null;
      stream?.close();
      login(false);
    }
    throw new Error(`${v.error?.code}: ${v.error?.message}`);
  }
  return v;
}
function notice(message) {
  const el = $("#connection");
  el.hidden = !message;
  el.textContent = message || "";
}
function toast(message, kind = "success") {
  const root = $("#toast"),
    item = document.createElement("div");
  item.className = `toast-item ${kind}`;
  item.setAttribute("role", kind === "error" ? "alert" : "status");
  const mark = document.createElement("span");
  mark.className = "toast-mark";
  mark.innerHTML = icon(kind === "error" ? "warning" : "check");
  const text = document.createElement("span");
  text.textContent = message;
  const dismiss = document.createElement("button");
  dismiss.type = "button";
  dismiss.className = "icon-button";
  dismiss.setAttribute("aria-label", "بستن پیام");
  dismiss.innerHTML = icon("close");
  dismiss.onclick = () => item.remove();
  item.append(mark, text, dismiss);
  root.append(item);
  while (root.children.length > 3) root.firstElementChild.remove();
  let timer = setTimeout(() => item.remove(), 6000);
  item.onmouseenter = item.onfocusin = () => clearTimeout(timer);
  item.onmouseleave = item.onfocusout = () => {
    timer = setTimeout(() => item.remove(), 6000);
  };
}
function close(refreshView = true) {
  const d = $("dialog");
  if (d) {
    d.close();
    d.remove();
  }
  const action = returnFocus?.dataset?.action,
    id = returnFocus?.dataset?.id;
  if (refreshView && user && $("#main")) render();
  if (returnFocus?.isConnected) returnFocus.focus();
  else if (action)
    document
      .querySelector(
        `[data-action="${CSS.escape(action)}"][data-id="${CSS.escape(id || "")}"]`,
      )
      ?.focus();
}
function modal(title, body, footer = "") {
  close(false);
  returnFocus = document.activeElement;
  $("#modal-root").innerHTML =
    `<dialog aria-labelledby="modal-title"><div class="modal-header"><h2 id="modal-title">${esc(title)}</h2><button class="icon-button" data-action="close" aria-label="بستن">${icon("close")}</button></div><div class="modal-body">${body}<p id="form-error" class="form-error" role="alert"></p></div>${footer ? `<div class="modal-footer">${footer}</div>` : ""}</dialog>`;
  const d = $("dialog");
  d.addEventListener("cancel", (e) => {
    e.preventDefault();
    close();
  });
  d.addEventListener("close", () => {
    if (returnFocus?.isConnected) returnFocus.focus();
  });
  d.addEventListener("click", (e) => {
    const box = d.getBoundingClientRect();
    if (
      e.target === d &&
      (e.clientX < box.left ||
        e.clientX > box.right ||
        e.clientY < box.top ||
        e.clientY > box.bottom)
    )
      close();
  });
  d.addEventListener("keydown", (e) => {
    if (e.key !== "Tab") return;
    const f = [
      ...d.querySelectorAll(
        'button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),a[href],summary,[tabindex="0"]',
      ),
    ].filter((x) => x.getClientRects().length);
    if (!f.length) {
      e.preventDefault();
      return;
    }
    const first = f[0],
      last = f.at(-1);
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  });
  d.showModal();
}
function form(title, body, submit, submitLabel = "ذخیره") {
  modal(
    title,
    `<form id="dialog-form"><div class="form-grid">${body}</div><button type="submit" class="btn primary">${submitLabel}</button></form>`,
  );
  $("#dialog-form").onsubmit = async (e) => {
    e.preventDefault();
    const b = e.target.querySelector("[type=submit]");
    b.disabled = true;
    const original = b.innerHTML;
    b.textContent = "در حال انجام…";
    b.classList.add("is-loading");
    b.setAttribute("aria-busy", "true");
    $("#form-error").textContent = "";
    try {
      await submit(Object.fromEntries(new FormData(e.target)), e.target);
    } catch (err) {
      if ($("#form-error")) $("#form-error").textContent = err.message;
    } finally {
      if (b.isConnected) {
        b.disabled = false;
        b.innerHTML = original;
        b.classList.remove("is-loading");
        b.removeAttribute("aria-busy");
      }
    }
  };
}
function login(bootstrap) {
  close();
  $("#main").innerHTML = "";
  $("#shell").hidden = true;
  $("#auth").hidden = false;
  $("#auth").innerHTML =
    `<main class="auth-card"><div class="brand"><span class="brand-mark">G</span><b dir="ltr">GreFlow</b></div><span class="auth-eyebrow">${bootstrap ? "شروع در یک قدم" : "خوش برگشتید"}</span><h1>${bootstrap ? "رمز پنل را تعیین کنید" : "ورود به GreFlow"}</h1><p class="help">${bootstrap ? "یک رمز برای ورودهای بعدی انتخاب کنید؛ سپس پنل آماده است." : "با رمز پنل وارد شوید و سرورها را مدیریت کنید."}</p><form id="login-form">${field("password", "رمز عبور", "", "password", `required minlength="8" autocomplete="${bootstrap ? "new-password" : "current-password"}"`)}${bootstrap ? field("confirm_password", "تکرار رمز عبور", "", "password", 'required minlength="8" autocomplete="new-password"') : ""}<details class="auth-advanced"><summary>${bootstrap ? "نام کاربری (پیش‌فرض: admin)" : "ورود با کاربر دیگر"}</summary>${field("name", "نام کاربری", "admin", "text", 'required autocomplete="username"')}</details><p id="login-error" class="form-error" role="alert"></p><button class="btn primary" type="submit">${bootstrap ? "شروع کار با GreFlow" : "ورود به پنل"}</button></form></main>`;
  $("#login-form").onsubmit = async (e) => {
    e.preventDefault();
    const b = e.target.querySelector('[type="submit"]');
    b.disabled = true;
    b.setAttribute("aria-busy", "true");
    const v = Object.fromEntries(new FormData(e.target));
    try {
      if (bootstrap && v.password !== v.confirm_password)
        throw new Error("تکرار رمز با رمز عبور یکسان نیست.");
      const credentials = { name: v.name, password: v.password };
      if (bootstrap) await api("bootstrap", credentials);
      await api("auth/login", credentials);
      await start();
      if (bootstrap) toast("GreFlow آماده است. اولین سرور را اضافه کنید.");
    } catch (err) {
      if ($("#login-error")) $("#login-error").textContent = err.message;
    } finally {
      if (b.isConnected) {
        b.disabled = false;
        b.removeAttribute("aria-busy");
      }
    }
  };
}
function check(t, kind) {
  const e = t.checks?.find((v) => v.kind === kind);
  if (!e || e.status === "unknown") return "unknown";
  if (e.status === "unsupported") return "unsupported";
  return new Date(e.expires_at) < new Date() ? "stale" : e.status;
}
function state(t) {
  if (
    t.checks?.some(
      (e) =>
        e.kind !== "user_verified" &&
        ["ok", "failed"].includes(e.status) &&
        new Date(e.expires_at) < new Date(),
    )
  )
    return "stale";
  if (check(t, "interface") === "failed" || check(t, "rules") === "failed")
    return "down";
  const ds = t.checks?.filter((e) => e.kind.startsWith("destination:")) || [];
  if (check(t, "gre_peer") === "failed" && ds.some((e) => e.status === "ok"))
    return "degraded";
  if (
    check(t, "gre_peer") === "ok" &&
    ds.length &&
    ds.every((e) => e.status === "ok")
  )
    return "healthy";
  if (
    check(t, "gre_peer") === "failed" ||
    ds.some((e) => e.status === "failed")
  )
    return "down";
  return "unknown";
}
function tunnels() {
  return hosts
    .flatMap((h) =>
      (h.inventory?.tunnels || []).map((t) => ({
        ...t,
        host_id: h.id,
        host_name: h.name,
        id: h.id + ":" + t.name,
      })),
    )
    .filter((t) => !selected() || t.host_id === selected());
}
const getTunnel = (id) =>
  hosts
    .flatMap((h) =>
      (h.inventory?.tunnels || []).map((t) => ({
        ...t,
        host_id: h.id,
        host_name: h.name,
        id: h.id + ":" + t.name,
      })),
    )
    .find((t) => t.id === id);
function evidenceLabel(kind, status) {
  if (kind === "user_verified")
    return (
      {
        ok: "تأیید کلاینت ثبت شد",
        stale: "تأیید قبلی",
        unknown: "هنوز ثبت نشده",
      }[status] || labels[status]
    );
  if (status === "stale") return "نیاز به بررسی";
  if (status === "unknown") return "بررسی نشده";
  if (status === "unsupported") return "نیاز به تست برنامه";
  if (kind === "rules") return status === "ok" ? "برقرار" : "نیاز به اصلاح";
  if (kind === "service") return status === "ok" ? "فعال" : "غیرفعال";
  if (kind === "gre_peer") return status === "ok" ? "پاسخ می‌دهد" : "بدون پاسخ";
  return status === "ok" ? "در دسترس" : "بدون دسترسی";
}
function evidenceMarkup(t, kind, compact = false) {
  const e = t.checks?.find((v) => v.kind === kind),
    status = check(t, kind);
  let note = "";
  if (kind === "user_verified")
    note =
      status === "unknown"
        ? "پس از تست واقعی برنامه ثبت کنید"
        : `زمان تأیید: ${time(e?.observed_at)}؛ تضمین اتصال فعلی نیست`;
  else if (status === "unknown") note = "بررسی اتصال را اجرا کنید";
  else if (status === "unsupported")
    note = "این مسیر به تست UDP یا برنامه نیاز دارد";
  else if (e)
    note = `${status === "stale" ? "آخرین نتیجه: " + evidenceLabel(kind, e.status) + " · " : "بررسی: "}${time(e.observed_at)}`;
  return `<span class="evidence" title="${esc(note)}">${badge(status, evidenceLabel(kind, status))}${compact ? "" : `<small>${esc(note)}</small>`}</span>`;
}
function layers(t) {
  return `<div class="health-grid">${[
    ["service", "سرویس تانل"],
    ["gre_peer", "پاسخ GRE"],
    ["rules", "هدایت پورت"],
  ]
    .map(
      ([k, l]) => `<div><small>${l}</small>${evidenceMarkup(t, k, true)}</div>`,
    )
    .join(
      "",
    )}</div><div class="tag-line">${(t.desired.forwards || []).map((f) => `<span class="port-evidence"><bdi>${esc(f.protocol.toUpperCase() + " " + f.public + " → " + f.destination)}</bdi>${evidenceMarkup(t, "destination:" + f.protocol + ":" + f.public, true)}</span>`).join("")}</div>`;
}
const ipInfoCache = new Map(),
  ipInfoPending = new Set();
const outsideIP = (t) =>
  t.desired.role === "entry" ? t.desired.remote : t.desired.local;
const iranIP = (t) =>
  t.desired.role === "entry" ? t.desired.local : t.desired.remote;
function geoMarkup(ip) {
  const g = ipInfoCache.get(ip);
  if (!g) return `<span class="geo-pending">در حال دریافت اطلاعات IP…</span>`;
  if (!g.country_code)
    return `<span class="muted">موقعیت نامشخص</span><small>${esc(g.error || "اطلاعات موجود نیست")}</small>`;
  const cc = /^[a-z]{2}$/.test(g.country_code) ? g.country_code : "";
  let country = g.country;
  try {
    if (cc)
      country = new Intl.DisplayNames(["fa"], { type: "region" }).of(
        cc.toUpperCase(),
      );
  } catch {}
  return `<div class="geo-place">${cc ? `<img src="/assets/flags/${cc}.svg" class="country-flag" alt="پرچم ${esc(country)}" width="32" height="24">` : ""}<b>${esc(country)}</b><span>${esc(g.city || g.region || "شهر نامشخص")}</span>${g.status === "stale" ? badge("stale", "اطلاعات قدیمی") : ""}</div><small class="geo-provider">شبکه / دیتاسنتر: <bdi>${esc(g.organization || "نامشخص")}</bdi>${g.asn ? ` · <bdi>${esc(g.asn)}</bdi>` : ""}</small><details class="geo-source" data-disclosure="geo:${esc(ip)}"><summary>منبع و زمان بررسی</summary><small>${esc(g.source)} · ${time(g.observed_at)}<br>موقعیت IP تقریبی است؛ نام شبکه الزاماً مالک فیزیکی دیتاسنتر نیست.${g.country_conflict ? " پایگاه‌های Check-Host دربارهٔ کشور اختلاف دارند." : ""}${g.error ? `<br>${esc(g.error)}` : ""}</small></details>`;
}
function geoBlock(t) {
  return `<div class="geo-block" data-geo-ip="${esc(outsideIP(t))}">${geoMarkup(outsideIP(t))}</div>`;
}
function loadIPInfo(ts) {
  for (const t of ts) {
    const ip = outsideIP(t),
      cached = ipInfoCache.get(ip);
    if (
      ipInfoPending.has(ip) ||
      (cached && Date.parse(cached.expires_at || 0) > Date.now())
    )
      continue;
    ipInfoPending.add(ip);
    api(
      `ip-info?host_id=${encodeURIComponent(t.host_id)}&tunnel=${encodeURIComponent(t.name || "")}`,
    )
      .then((info) => ipInfoCache.set(ip, info))
      .catch((e) =>
        ipInfoCache.set(ip, {
          status: "unavailable",
          error: e.message,
          expires_at: new Date(Date.now() + 300000).toISOString(),
        }),
      )
      .finally(() => {
        ipInfoPending.delete(ip);
        document.querySelectorAll("[data-geo-ip]").forEach((el) => {
          if (el.dataset.geoIp === ip) el.innerHTML = geoMarkup(ip);
        });
      });
  }
}
function checkHostLink(t) {
  return `<a class="btn" href="https://check-host.net/ip-info?host=${encodeURIComponent(outsideIP(t))}" target="_blank" rel="noopener noreferrer">Check‑Host ↗</a>`;
}
function card(t) {
  const outside = outsideIP(t);
  return `<article class="tunnel-card" aria-label="تانل ${esc(t.name || "default")} به ${esc(outside)}"><div class="card-top"><span class="tunnel-label">${icon("tunnels")} ${esc(t.name || "default")}</span>${badge(state(t))}</div><div class="endpoint-heading"><div><small>سرور خارج</small><h3 class="mono">${esc(outside)}</h3></div>${btn("کپی IP", "copy-ip", outside)}</div>${geoBlock(t)}<div class="route-strip"><span>ایران <bdi class="mono">${esc(iranIP(t))}</bdi></span>${icon("tunnels")}<span>خارج <bdi class="mono">${esc(outside)}</bdi></span></div><div class="tag-line port-tags">${(t.desired.forwards || []).map((f) => `<span class="chip mono">${esc(f.protocol.toUpperCase())} ${esc(f.public)} → ${esc(f.destination)}</span>`).join("") || '<span class="chip">بدون پورت</span>'}</div>${layers(t)}<div class="card-meta"><span>دریافت <bdi class="mono">${t.rx_bytes == null ? "—" : t.rx_bytes.toLocaleString()} B</bdi></span><span>ارسال <bdi class="mono">${t.tx_bytes == null ? "—" : t.tx_bytes.toLocaleString()} B</bdi></span></div><p class="compact-note">آخرین مشاهده: ${time(t.checks?.[0]?.observed_at)}</p><div class="card-footer">${btn("مدیریت تانل", "detail", t.id, true)}${user.role === "viewer" ? "" : btn("بررسی اتصال", "probe", t.id)}${btn("لاگ", "logs", t.id)}</div><div class="card-links">${checkHostLink(t)}</div></article>`;
}
function hostPicker() {
  return `<select id="host-filter" aria-label="فیلتر سرور"><option value="">همهٔ سرورها</option>${hosts.map((h) => `<option value="${h.id}" ${selected() === h.id ? "selected" : ""}>${esc(h.name)}</option>`).join("")}</select>`;
}
function overview() {
  const ts = tunnels(),
    states = ts.map(state);
  return (
    heading(
      "نمای کلی",
      "وضعیت واقعی زیرساخت، با زمان مشاهده و تفکیک لایه‌ها",
      user.role === "admin"
        ? btn(icon("plus") + " افزودن سرور", "add-host", "", true)
        : "",
    ) +
    `<div class="summary-grid">${[
      ["سرور ثبت‌شده", hosts.length, "servers"],
      ["تانل شناسایی‌شده", ts.length, "tunnels"],
      [
        "نیازمند بررسی",
        states.filter((s) => ["down", "degraded"].includes(s)).length,
        "warning",
      ],
      [
        "قدیمی یا نامشخص",
        states.filter((s) => ["stale", "unknown"].includes(s)).length,
        "schedules",
      ],
    ]
      .map(
        ([label, n, i]) =>
          `<div class="summary-card"><span class="summary-icon">${icon(i)}</span><div><strong>${n.toLocaleString("fa-IR")}</strong><b>${label}</b></div></div>`,
      )
      .join(
        "",
      )}</div><div class="section-title"><h2>سرورهای شما</h2><a href="#servers">مشاهدهٔ همه</a></div>${hosts.length ? `<div class="server-grid">${hosts.map(hostCard).join("")}</div>` : empty("اولین سرور را متصل کنید", "با پسورد یا کلید SSH متصل شوید. شناسایی اولیه تنظیمات سرور را تغییر نمی‌دهد.", user.role === "admin" ? btn("افزودن سرور", "add-host", "", true) : "")}`
  );
}
function hostCard(h) {
  const latest = operations.find((o) => o.host_id === h.id);
  const busy = operations.find(
    (o) => o.host_id === h.id && ["queued", "running"].includes(o.state),
  );
  const admin = user.role === "admin",
    ssh = h.hostname !== "agent",
    ready = !!h.inventory;
  return `<article class="server-card"><div class="card-top"><h3>${esc(h.name)}</h3><span class="chip blue">${h.kind === "iran" ? "ایران" : "خارج"} · ${ssh ? "SSH" : "mTLS"}</span></div><p class="mono">${ssh ? esc(h.hostname) + ":" + h.port : "عامل خروجی"}</p><div class="server-summary"><b>${ready ? h.inventory.tunnels.length.toLocaleString("fa-IR") + " تانل شناسایی‌شده" : "نیازمند آماده‌سازی"}</b><small>آخرین شناسایی: ${time(h.observed_at)}</small></div>${h.last_error ? `<p class="form-error" role="alert">${esc(h.last_error)}</p>` : ""}${busy ? `<p class="server-progress" role="status">${esc(busy.result?.diagnostics?.at(-1)?.message || "عملیات در صف است…")}</p>` : ""}<div class="card-footer">${!ready && admin && ssh && !busy ? btn("آماده‌سازی خودکار", "prepare", h.id, true) : btn("مدیریت تانل‌ها", "host", h.id, true)}${user.role !== "viewer" && !busy ? btn("شناسایی فقط‌خواندنی", "discover", h.id) : ""}${latest ? btn(busy ? "مشاهدهٔ پیشرفت" : "گزارش اتصال و عملیات", "operation", (busy || latest).id) : ""}</div><details class="server-tools" data-disclosure="host:${esc(h.id)}"><summary>ابزارها و جزئیات سرور</summary><div class="actions">${ssh && user.role !== "viewer" ? btn("اطلاعات سیستم", "server_info", h.id) + btn("لاگ SSH", "server_logs", h.id) : ""}${admin && ssh ? btn("بررسی و آماده‌سازی ابزار", "prepare", h.id) + btn("تغییر دسترسی SSH", "credentials", h.id) + `<button class="btn danger" data-action="server-reboot" data-id="${esc(h.id)}">ری‌استارت سرور</button>` : ""}</div>${h.addresses?.length ? `<p class="compact-note">آدرس‌های شناسایی‌شده</p><div class="tag-line">${h.addresses.map((v) => `<span class="chip mono">${esc(v)}</span>`).join("")}</div>` : ""}</details></article>`;
}
function tunnelPage() {
  return (
    heading(
      "مدیریت تانل‌ها",
      "سرویس فعال و TCP موفق به‌تنهایی اتصال واقعی کانفیگ را تأیید نمی‌کنند.",
      user.role === "admin"
        ? btn(icon("plus") + " افزودن تانل", "wizard", "", true)
        : "",
    ) +
    `<div class="toolbar">${hostPicker()}<div class="search"><input id="search" aria-label="جست‌وجوی نام، آی‌پی یا پورت" placeholder="جست‌وجوی نام، آی‌پی یا پورت…" value="${esc(params().get("q") || "")}"></div><select id="state-filter" aria-label="فیلتر وضعیت"><option value="">همهٔ وضعیت‌ها</option>${["healthy", "degraded", "down", "unknown", "stale"].map((s) => `<option value="${s}" ${params().get("status") === s ? "selected" : ""}>${labels[s]}</option>`).join("")}</select><div class="segments">${btn("کارتی", "cards")}${btn("جدولی", "table")}</div></div><div id="results"></div>`
  );
}
function renderResults() {
  const el = $("#results");
  if (!el) return;
  const q = (params().get("q") || "").toLowerCase(),
    filter = params().get("status");
  const all = tunnels().filter(
    (t) =>
      (!filter || state(t) === filter) &&
      JSON.stringify([t.name, t.desired]).toLowerCase().includes(q),
  );
  const pg = Math.max(1, Number(params().get("p")) || 1),
    ts = all.slice((pg - 1) * 24, pg * 24);
  loadIPInfo(ts);
  el.innerHTML =
    `<p class="compact-note">${all.length.toLocaleString("fa-IR")} نتیجه · صفحهٔ ${pg.toLocaleString("fa-IR")}</p>` +
    (ts.length
      ? params().get("view") === "table"
        ? `<div class="table-wrap"><table><thead><tr><th>تانل</th><th>خارج</th><th>پورت‌ها</th><th>وضعیت</th><th>جزئیات</th></tr></thead><tbody>${ts.map((t) => `<tr><td class="mono">${esc(t.name || "default")}</td><td><bdi class="mono">${esc(outsideIP(t))}</bdi>${geoBlock(t)}${checkHostLink(t)}</td><td class="mono">${esc((t.desired.forwards || []).map((f) => `${f.protocol} ${f.public} → ${f.destination}`).join(" / "))}</td><td>${badge(state(t))}</td><td>${btn("مشاهده", "detail", t.id)}</td></tr>`).join("")}</tbody></table></div>`
        : `<div class="cards">${ts.map(card).join("")}</div>`
      : empty(
          "تانلی پیدا نشد",
          "سرور را شناسایی کنید یا فیلترها را تغییر دهید.",
        )) +
    `<div class="actions">${pg > 1 ? btn("صفحهٔ قبل", "prev") : ""}${all.length > pg * 24 ? btn("صفحهٔ بعد", "next") : ""}</div>`;
}
function portsPage() {
  const rows = tunnels()
    .filter((t) => t.desired.role === "entry")
    .flatMap((t) => (t.desired.forwards || []).map((f) => ({ t, f })));
  return (
    heading(
      "مسیر پورت‌ها",
      "هدایت پورت یعنی تنظیم فایروال؛ تست مقصد فقط باز بودن پورت خارج را می‌سنجد. تست واقعی برنامه را خودتان تأیید می‌کنید.",
      hostPicker() +
        (user.role === "admin" ? btn("پایش خودکار", "monitor") : ""),
    ) +
    (rows.length
      ? `<div class="table-wrap"><table><thead><tr><th>ایران</th><th>ورودی</th><th>تانل / خارج</th><th>مقصد</th><th>هدایت پورت</th><th>دسترسی مقصد</th><th>تست واقعی کلاینت</th><th>عملیات</th></tr></thead><tbody>${rows.map(({ t, f }) => `<tr><td class="mono">${esc(t.desired.local)}</td><td class="mono">${esc(f.protocol)} ${esc(f.public)}</td><td><b class="mono">${esc(t.name || "default")}</b><p class="mono">${esc(t.desired.remote)}</p></td><td class="mono">${esc(f.destination)}</td><td>${evidenceMarkup(t, "rules")}</td><td>${evidenceMarkup(t, `destination:${f.protocol}:${f.public}`)}</td><td>${evidenceMarkup(t, "user_verified")}</td><td>${btn("جزئیات", "detail", t.id)}${user.role === "viewer" ? "" : btn("بررسی اتصال", "probe", t.id) + btn("انتقال", "transfer", `${t.id}|${f.protocol}|${f.public}`)}</td></tr>`).join("")}</tbody></table></div>`
      : empty(
          "پورتی ثبت نشده",
          "پس از شناسایی یا افزودن تانل، مسیرها اینجا نمایش داده می‌شوند.",
        ))
  );
}
function operationLabel(o) {
  return (
    {
      prepare: "آماده‌سازی خودکار",
      discover: "شناسایی تانل‌ها",
      observe: "خواندن وضعیت",
      probe: "بررسی اتصال",
      logs: "دریافت لاگ تانل",
      start: "شروع تانل",
      stop: "توقف تانل",
      restart: "ری‌استارت تانل",
      server_info: "اطلاعات سیستم",
      server_logs: "لاگ SSH",
      server_reboot: "ری‌استارت سرور",
      create: "افزودن تانل",
      mappings: "ویرایش پورت",
      transfer: "انتقال پورت",
      schedule: "زمان‌بندی",
      unschedule: "حذف زمان‌بندی",
      verify: "ثبت تأیید کلاینت",
      remove: "حذف تانل",
    }[o.request.action] || o.request.action
  );
}
function operationDetails(o) {
  const stages = {
    validation: "بررسی اطلاعات اتصال",
    network: "ارتباط شبکه",
    identity: "هویت سرور",
    authentication: "ورود SSH",
    handshake: "مذاکرهٔ SSH",
    ssh: "اتصال SSH",
    helper: "بررسی ابزار موجود",
    platform: "سیستم‌عامل و معماری",
    package: "فایل نصب",
    install: "نصب ابزار",
    verify: "تأیید نسخه",
    discover: "شناسایی تانل‌ها",
    dispatch: "اجرای عملیات",
    server_info: "اطلاعات سیستم",
    server_logs: "لاگ SSH",
    server_reboot: "درخواست ری‌استارت سرور",
  };
  const steps = [
    ...new Map((o.result?.diagnostics || []).map((d) => [d.stage, d])).values(),
  ];
  return `<p class="compact-note">${esc(hosts.find((h) => h.id === o.host_id)?.name || o.host_id)} · ${esc(operationLabel(o))}</p><div role="status">${badge(o.state)} ${esc(o.result?.diagnostics?.at(-1)?.message || "در انتظار شروع عملیات")}</div>${o.result?.error ? `<p class="form-error" role="alert">${esc(o.result.error)}</p>` : ""}<ol class="diagnostic-list">${steps.map((d) => `<li><div class="diagnostic-title"><b>${esc(stages[d.stage] || d.stage)}</b>${badge(d.state)}</div><p>${esc(d.message)}</p><small>${time(d.at)}${d.code ? ` · <bdi class="mono">${esc(d.code)}</bdi>` : ""}${d.exit_code !== undefined ? ` · کد خروج: ${d.exit_code}` : ""}</small></li>`).join("")}</ol>${o.result?.logs ? `<pre class="code-box" dir="ltr">${esc(o.result.logs)}</pre>` : ""}<details><summary>جزئیات فنی و شناسهٔ پیگیری</summary><pre class="code-box">${esc(JSON.stringify(o, null, 2))}</pre></details>`;
}
function showOperation(id) {
  const o = operations.find((o) => o.id === id);
  if (!o) return;
  modal(
    "گزارش اتصال و عملیات",
    `<div id="operation-report" data-operation="${esc(id)}">${operationDetails(o)}</div>`,
    btn("دانلود گزارش", "download-report", id),
  );
}
function operationsPage() {
  return (
    heading(
      "عملیات و نتیجه‌ها",
      "کار در صف سرور ادامه می‌یابد؛ بستن مرورگر آن را لغو نمی‌کند.",
    ) +
    `<div class="panel">${operations.length ? operations.map((o) => `<div class="list-row"><div class="operation-main"><b>${esc(operationLabel(o))} · <span>${esc(hosts.find((h) => h.id === o.host_id)?.name || o.request.name || "سرور")}</span></b><p>${time(o.updated_at)}</p><small class="mono">${esc(o.id)}</small>${o.result?.error ? `<p class="form-error">${esc(o.result.error)}</p>` : ""}</div>${badge(o.state)}${btn("جزئیات", "operation", o.id)}</div>`).join("") : empty("هنوز عملیاتی اجرا نشده", "عملیات تأییدشده همراه شواهد و خطا ذخیره می‌شوند.")}</div>`
  );
}
function schedulesPage() {
  const rows = schedules
    .map((v) => {
      const observation = hosts
        .find((h) => h.id === v.host_id)
        ?.inventory?.schedules?.find((o) => o.schedule.id === v.schedule.id);
      const fresh =
        observation &&
        Date.now() - Date.parse(observation.observed_at) < 180000;
      const status = !observation
        ? "unknown"
        : !fresh
          ? "stale"
          : observation.timer_state === "active"
            ? "ok"
            : observation.timer_state === "failed"
              ? "failed"
              : "unknown";
      return `<div class="list-row"><div><b>${esc(v.schedule.name || "default")} · ${esc(v.schedule.action)}</b><p>هر ${v.schedule.minutes} دقیقه · ${esc(v.schedule.timezone)}</p><p>خواستهٔ ذخیره‌شده: ${v.schedule.enabled ? "فعال" : "غیرفعال"} · وضعیت خوانده‌شده: ${badge(status)}</p><p>اجرای بعدی میزبان: ${fresh && observation.next_run_at ? time(observation.next_run_at) : "نامشخص؛ نیاز به شناسایی و systemd سازگار"}</p>${v.schedule.maintenance_windows?.length ? '<span class="chip">پنجرهٔ نگهداری روزانه</span>' : ""}</div><div class="actions">${btn("نتایج اجرا", "schedule-runs", v.schedule.id)}${user.role === "viewer" ? "" : `${btn("خواندن وضعیت میزبان", "discover", v.host_id)}${btn(v.schedule.enabled ? "غیرفعال‌کردن" : "فعال‌کردن", "toggle-schedule", v.schedule.id)}${btn("حذف", "delete-schedule", v.schedule.id)}${btn(v.schedule.maintenance ? "پایان نگهداری" : "حالت نگهداری", "maintenance-schedule", v.schedule.id)}`}</div></div>`;
    })
    .join("");
  return (
    heading(
      "زمان‌بندی روی میزبان",
      "timer روی سرور اجرا می‌شود؛ خاموشی پنل آن را متوقف نمی‌کند.",
      user.role === "viewer" ? "" : btn("برنامهٔ جدید", "schedule", "", true),
    ) +
    `<div class="notice">بازیابی شرطی علاوه بر سقف هر برنامه، سقف مشترک ۶ تلاش در ساعت و حداقل فاصلهٔ ۵ دقیقه روی میزبان دارد. پنجرهٔ نگهداری طبق منطقهٔ زمانی اجرا می‌شود.</div><div class="panel">${rows || empty("برنامه‌ای وجود ندارد", "پس از ثبت، تأیید میزبان در عملیات دیده می‌شود.")}</div>`
  );
}

function settingsPage() {
  return (
    heading("تنظیمات و امنیت", "ترجیحات نمایش، دسترسی‌ها و بازیابی اطلاعات") +
    `<div class="settings-stack"><section class="panel"><h2>ظاهر و زمان</h2><div class="form-grid">${select(
      "theme",
      "پوسته",
      [
        ["system", "همگام با سیستم"],
        ["light", "روشن"],
        ["dark", "تاریک"],
      ],
      localStorage.getItem("greflow-theme") || "system",
    )}${field("timezone", "منطقهٔ زمانی", localStorage.getItem("greflow-timezone") || "Asia/Tehran")}</div>${btn("اعمال تنظیمات نمایش", "preferences")}</section><section class="panel"><h2>ورود و دسترسی</h2><p class="help">رمز پنل، نشست‌های فعال و دسترسی کاربران.</p><div class="actions">${btn("نشست‌های من", "sessions")}${btn("تغییر رمز عبور", "password")}${btn("لغو همهٔ نشست‌های من", "revoke")}</div>${user.role === "admin" ? btn("مدیریت کاربران", "users") : ""}</section>${user.role === "admin" ? `<section class="panel"><h2>پشتیبان مدیریت</h2><p class="help">فایل اختصاصی GreFlow شامل تنظیمات، سرورها، دسترسی‌ها و تاریخچه است. فقط همین پروژه آن را می‌خواند؛ کل فایل با رمز بکاپ محافظت می‌شود.</p><div class="actions">${btn("ساخت بکاپ", "backup")}${btn("پیش‌نمایش بازیابی", "restore")}${btn("تاریخچهٔ دسترسی", "audit")}</div></section><section class="panel"><h2>هشدار تلگرام</h2><p class="help">صف اعلان با جلوگیری از تکرار، تلاش مجدد و پیام بازیابی. ارسال مرکزی فقط تا وقتی برنامهٔ پنل روشن است ادامه دارد.</p>${btn("تنظیم مقصد", "notifications")}${btn("پایش و نگهداری تاریخچه", "monitor")}${btn("عامل‌ها و لغو دسترسی", "agents")}</section>` : ""}<section class="panel"><h2>دربارهٔ این ساخت</h2><p>رابط مشترک ویندوز و لینوکس · موتور GRE نسخهٔ ۰٫۲</p><p>عامل خروجی mTLS: ${esc(capabilities.push_agent_reason || "در دسترس نیست")}</p><p class="compact-note">نسخهٔ کنسول: <bdi>${esc(capabilities.build || "dev")}</bdi> · دامنهٔ تست‌ها و محدودیت‌ها در راهنمای همراه بسته ثبت شده‌اند.</p></section></div>`
  );
}
let renderedPage = "";
function render() {
  if (!user) return;
  const p = page();
  const samePage = renderedPage === p;
  const opened = new Set(
    samePage
      ? [
          ...document.querySelectorAll("#main details[open][data-disclosure]"),
        ].map((el) => el.dataset.disclosure)
      : [],
  );
  const active = document.activeElement;
  let focusSelector = "";
  if (samePage && active?.closest("#main")) {
    if (active.id) focusSelector = `#${CSS.escape(active.id)}`;
    else if (active.dataset.action)
      focusSelector = `[data-action="${CSS.escape(active.dataset.action)}"][data-id="${CSS.escape(active.dataset.id || "")}"]`;
    else if (
      active.tagName === "SUMMARY" &&
      active.parentElement.dataset.disclosure
    )
      focusSelector = `[data-disclosure="${CSS.escape(active.parentElement.dataset.disclosure)}"] > summary`;
  }
  $("#navigation").innerHTML = nav
    .map(
      ([k, l]) =>
        `<a class="nav-link ${p === k ? "active" : ""}" href="#${k}" ${p === k ? 'aria-current="page"' : ""}>${icon(k)}${l}</a>`,
    )
    .join("");
  $("#breadcrumb").textContent = nav.find(([k]) => k === p)?.[1] || "نمای کلی";
  $("#identity").textContent = user.name;
  const views = {
    overview,
    servers: () =>
      heading(
        "سرورها",
        "هر میزبان می‌تواند چند آدرس داشته باشد؛ هویت میزبان پس از شناسایی بررسی می‌شود.",
        user.role === "admin"
          ? btn("افزودن SSH", "add-host", "", true) +
              btn("عامل خروجی", "enroll")
          : "",
      ) + `<div class="server-grid">${hosts.map(hostCard).join("")}</div>`,
    tunnels: tunnelPage,
    ports: portsPage,
    operations: operationsPage,
    schedules: schedulesPage,
    settings: settingsPage,
  };
  $("#main").innerHTML = (views[p] || overview)();
  renderResults();
  document.querySelectorAll("#main details[data-disclosure]").forEach((el) => {
    el.open = opened.has(el.dataset.disclosure);
  });
  if (focusSelector)
    $("#main").querySelector(focusSelector)?.focus({ preventScroll: true });
  renderedPage = p;
}
async function refresh() {
  if (refreshing || !user) return;
  refreshing = true;
  try {
    [hosts, operations, schedules] = await Promise.all([
      api("servers"),
      api("operations"),
      api("schedules"),
    ]);
    notice("");
    if (
      !$("dialog") &&
      !["INPUT", "SELECT", "TEXTAREA"].includes(
        document.activeElement?.tagName,
      ) &&
      (page() !== "settings" || !$("#main").children.length)
    )
      render();
    const report = $("#operation-report");
    if (report) {
      const operation = operations.find(
        (o) => o.id === report.dataset.operation,
      );
      if (operation && report.dataset.revision !== operation.updated_at) {
        const open = report.querySelector("details")?.open;
        report.innerHTML = operationDetails(operation);
        if (open) report.querySelector("details").open = true;
        report.dataset.revision = operation.updated_at;
      }
    }
  } catch (e) {
    notice(
      "دریافت اطلاعات ناموفق: " +
        e.message +
        "؛ وضعیت‌های قبلی فقط با زمان مشاهده معتبرند.",
    );
  } finally {
    refreshing = false;
  }
}
async function queue(host_id, request) {
  const o = await api("operations", {
    host_id,
    request: { version: 1, id: uid(), ...request },
  });
  toast("عملیات در صف ثبت شد.");
  await refresh();
  return o.id;
}
async function waitOperation(id) {
  for (let i = 0; i < 90; i++) {
    const ops = await api("operations");
    const o = ops.find((v) => v.id === id);
    if (o && !["queued", "running"].includes(o.state)) return o;
    await new Promise((r) => setTimeout(r, 1500));
  }
  throw new Error(
    "مهلت انتظار مرورگر تمام شد؛ نتیجه را در عملیات پیگیری کنید.",
  );
}
function detail(id) {
  const t = getTunnel(id);
  if (!t) return;
  loadIPInfo([t]);
  modal(
    `جزئیات ${t.name || "default"} · ${outsideIP(t)}`,
    `<div class="endpoint-heading"><div><small>سرور خارج</small><h3 class="mono">${esc(outsideIP(t))}</h3></div>${btn("کپی IP", "copy-ip", outsideIP(t))}</div>${geoBlock(t)}<div class="card-links">${checkHostLink(t)}</div>${layers(t)}<p class="notice">تست TCP فقط باز بودن مقصد را می‌سنجد. تأیید واقعی کانفیگ به تست کلاینت نیاز دارد.</p><div class="list-row"><b>تست واقعی کلاینت</b>${evidenceMarkup(t, "user_verified")}</div><div class="list-row"><b>اختلاف ذخیره و اعمال</b>${badge(t.drift ? "failed" : "ok", t.drift ? "اختلاف مشاهده شد" : "یکسان")}</div><div class="actions">${user.role === "viewer" ? "" : ["start", "stop", "restart", "mappings", "schedule", "verify"].map((a) => btn({ start: "شروع", stop: "توقف", restart: "ری‌استارت", mappings: "ویرایش پورت‌ها", schedule: "زمان‌بندی", verify: "ثبت تأیید کلاینت" }[a], a, t.id)).join("")}${btn("لاگ واقعی", "logs", t.id)}</div><details><summary>پیکربندی ذخیره‌شده و اعمال‌شده</summary><pre class="code-box">${esc(JSON.stringify({ desired: t.desired, applied: t.applied, revision: t.revision }, null, 2))}</pre></details>${user.role === "admin" ? btn("حذف تانل", "remove", t.id) : ""}`,
  );
}
function sshCredentialFields(method = "password") {
  return `<div class="full">${select(
    "auth_method",
    "روش ورود",
    [
      ["password", "پسورد SSH"],
      ["key", "کلید خصوصی SSH"],
    ],
    method,
  )}</div><div id="ssh-password-fields" class="full" ${method === "key" ? "hidden" : ""}>${field("password", "پسورد SSH", "", "password", `autocomplete="new-password" ${method === "password" ? "required" : "disabled"}`)}</div><div id="ssh-key-fields" class="full" ${method !== "key" ? "hidden" : ""}><div class="field"><label for="private-key">کلید خصوصی SSH</label><textarea id="private-key" name="private_key" dir="ltr" rows="5" autocomplete="off" spellcheck="false" ${method === "key" ? "required" : "disabled"}></textarea><label class="key-upload">انتخاب فایل کلید<input id="ssh-key-file" type="file" aria-label="فایل کلید خصوصی"></label></div>${field("key_passphrase", "رمز کلید (اگر دارد)", "", "password", `autocomplete="new-password" ${method !== "key" ? "disabled" : ""}`)}</div>`;
}
function toggleSSHMethod() {
  const method = $("#f-auth_method").value;
  for (const [id, active] of [
    ["ssh-password-fields", method === "password"],
    ["ssh-key-fields", method === "key"],
  ]) {
    const section = document.getElementById(id);
    section.hidden = !active;
    section.querySelectorAll("input,textarea").forEach((f) => {
      f.disabled = !active;
      if (f.name === "password" || f.name === "private_key")
        f.required = active;
    });
  }
}
function addHost() {
  form(
    "اتصال سرور جدید",
    `<p class="help full">آدرس و اطلاعات ورود را وارد کنید. پس از افزودن، ابزار مدیریت در صورت نیاز خودکار نصب و تانل‌ها شناسایی می‌شوند؛ تنظیمات تانل‌ها حفظ می‌شوند.</p>` +
      field("name", "نام سرور", "", "text", 'required maxlength="100"') +
      select("kind", "نوع سرور", [
        ["iran", "ایران"],
        ["outside", "خارج"],
      ]) +
      `<div class="full">${field("hostname", "آدرس IP یا دامنه", "", "text", 'required dir="ltr"')}</div>` +
      `<details class="full ssh-advanced"><summary>تنظیمات SSH · root و پورت 22</summary><div class="form-grid">${field("username", "کاربر SSH", "root", "text", 'required dir="ltr"')}${field("port", "پورت SSH", "22", "number", 'required min="1" max="65535"')}</div></details>` +
      sshCredentialFields(),
    async (v) => {
      const host = { ...v, port: Number(v.port), addresses: [] };
      const activeForm = $("#dialog-form");
      const result = await api("ssh/check", host);
      if (!activeForm.isConnected) return;
      modal(
        "اتصال SSH برقرار شد",
        `<div class="connection-success">${icon("check")}<div><b>${esc(host.name)}</b><p class="mono">${esc(host.username)}@${esc(host.hostname)}:${host.port}</p></div></div><p>هویت این سرور در اولین اتصال ذخیره می‌شود؛ تغییر بعدی آن به شما اطلاع داده خواهد شد.</p>${result.helper_ready ? '<p class="help">عامل GreFlow نصب است؛ می‌توانید تانل‌ها را شناسایی کنید.</p>' : '<p class="notice">ورود SSH موفق بود. پس از افزودن سرور، ابزار مدیریت از بستهٔ همین کنسول منتقل و خودکار نصب می‌شود؛ سپس شناسایی اجرا می‌شود.</p>'}<details><summary>جزئیات فنی اتصال</summary><p class="mono fingerprint">${esc(result.fingerprint)}</p></details>`,
        btn("افزودن و آماده‌سازی خودکار", "save-ssh-host", "", true),
      );
      $("[data-action=save-ssh-host]").onclick = async (event) => {
        event.stopPropagation();
        const button = event.currentTarget;
        button.disabled = true;
        try {
          const saved = await api("servers", {
            ...host,
            fingerprint: result.fingerprint,
          });
          close();
          await refresh();
          toast("سرور اضافه شد؛ آماده‌سازی خودکار در حال انجام است.");
          if (saved.preparation_id) showOperation(saved.preparation_id);
        } catch (err) {
          if ($("#form-error")) $("#form-error").textContent = err.message;
          button.disabled = false;
        }
      };
    },
    "بررسی اتصال",
  );
}
function confirmAction(action, t) {
  const names = {
    start: "شروع",
    stop: "توقف",
    restart: "ری‌استارت",
    remove: "حذف",
    verify: "ثبت تأیید کلاینت",
  };
  form(
    `${names[action]} تانل`,
    `<div class="notice full">${action === "verify" ? "فقط پس از اتصال موفق کلاینت واقعی این تأیید را ثبت کنید." : "این عملیات روی میزبان اجرا می‌شود و ممکن است اتصال‌های همین تانل قطع شوند."}<br><b class="mono">${esc(t.desired.local)} · ${esc(t.name || "default")}</b></div>` +
      field(
        "confirm",
        "برای تأیید نام تانل را وارد کنید",
        "",
        "text",
        'required dir="ltr"',
      ),
    async (v) => {
      if (v.confirm !== (t.name || "default"))
        throw new Error("نام مطابقت ندارد.");
      await queue(t.host_id, { action, name: t.name, revision: t.revision });
      close();
      location.hash = "operations";
      render();
    },
    names[action],
  );
}
function mappings(t) {
  form(
    "ویرایش نگاشت پورت‌ها",
    `<div class="field full"><label for="mapping-lines">هر خط: protocol public destination</label><textarea id="mapping-lines" name="lines" dir="ltr" rows="8" spellcheck="false">${esc((t.desired.forwards || []).map((f) => `${f.protocol} ${f.public} ${f.destination}`).join("\n"))}</textarea></div><p class="help full">فقط TCP/UDP، پورت ۱ تا ۶۵۵۳۵ و بازهٔ همسان. حذف خط، فوروارد را حذف می‌کند؛ برنامهٔ مقصد حذف نمی‌شود. شبکه و رابط GRE حفظ می‌شوند.</p>`,
    async (v) => {
      const forwards = v.lines.trim()
        ? v.lines
            .trim()
            .split("\n")
            .map((line) => {
              const p = line.trim().split(/\s+/);
              if (p.length !== 3)
                throw new Error("هر خط باید سه مقدار داشته باشد.");
              return { protocol: p[0], public: p[1], destination: p[2] };
            })
        : [];
      await queue(t.host_id, {
        action: "mappings",
        name: t.name,
        revision: t.revision,
        config: { ...t.desired, forwards },
      });
      close();
      location.hash = "operations";
      render();
    },
    "اعمال نگاشت‌ها",
  );
}
function transfer(id) {
  const [tid, proto, pub] = id.split("|"),
    t = getTunnel(tid),
    f = t.desired.forwards.find(
      (f) => f.protocol === proto && f.public === pub,
    );
  if (proto !== "tcp" || pub.includes("-")) {
    modal(
      "انتقال پورت",
      "این ساخت فقط انتقال تک‌پورت TCP را با preflight واقعی پشتیبانی می‌کند. برای UDP یا بازه، probe برنامه لازم است.",
    );
    return;
  }
  const targets = tunnels().filter(
    (x) =>
      x.host_id === t.host_id &&
      x.name !== t.name &&
      x.desired.local === t.desired.local,
  );
  form(
    "انتقال پورت با برگشت",
    select(
      "target",
      "تانل مقصد",
      targets.map((x) => [
        x.id,
        (x.name || "default") + " · " + x.desired.remote,
      ]),
    ) +
      field(
        "destination",
        "پورت مقصد جدید",
        f.destination,
        "number",
        'required min="1" max="65535"',
      ) +
      `<div class="notice full">${esc(t.desired.local)}:${esc(pub)} → ${esc(t.desired.remote)}:${esc(f.destination)}<br>ابتدا مقصد جدید تست می‌شود. در شکست بعد از تغییر، برگشت محدود به قوانین متعلق به این دو تانل انجام می‌شود. نشست‌های قبلی ممکن است تا reconnect روی مسیر قبلی بمانند.</div>`,
    async (v) => {
      const target = targets.find((x) => x.id === v.target);
      if (!target) throw new Error("تانل مقصد لازم است.");
      await queue(t.host_id, {
        action: "transfer",
        name: t.name,
        revision: t.revision,
        target: target.name,
        target_revision: target.revision,
        forward: f,
        destination: v.destination,
      });
      close();
      location.hash = "operations";
      render();
    },
    "تست مقصد و انتقال",
  );
}
function schedule(t) {
  const ts = tunnels();
  form(
    "برنامهٔ میزبان",
    select(
      "target",
      "تانل",
      ts.map((x) => [x.id, (x.name || "default") + " · " + x.host_name]),
      t?.id,
    ) +
      select("action", "عملیات", [
        ["probe", "بررسی دوره‌ای"],
        ["restart", "ری‌استارت برنامه‌ریزی‌شده"],
        [
          "recovery",
          "بازیابی شرطی (۳ شکست، فاصلهٔ ۱۰ دقیقه، سقف ۳ بار در ساعت)",
        ],
      ]) +
      field(
        "minutes",
        "فاصلهٔ اجرا به دقیقه",
        "30",
        "number",
        'required min="1" max="10080"',
      ) +
      field(
        "timezone",
        "منطقهٔ زمانی نگهداری و نمایش",
        "Asia/Tehran",
        "text",
        'required dir="ltr"',
      ) +
      field("maintenance_start", "شروع نگهداری روزانه (اختیاری)", "", "time") +
      field("maintenance_end", "پایان نگهداری روزانه (اختیاری)", "", "time") +
      `<p class="help full">systemd اجرای هم‌زمان یک برنامه را جلوگیری می‌کند. اجرای بعدی پس از شروع timer با تأخیر تصادفی حداکثر ۱۵ ثانیه است. نتیجهٔ هر اجرا در journal میزبان ثبت می‌شود.</p>`,
    async (v) => {
      const target = ts.find((x) => x.id === v.target);
      if (!target) throw new Error("تانل لازم است.");
      const windows = [];
      if (v.maintenance_start || v.maintenance_end) {
        if (
          !v.maintenance_start ||
          !v.maintenance_end ||
          v.maintenance_start === v.maintenance_end
        )
          throw new Error("شروع و پایان متفاوت برای پنجرهٔ نگهداری لازم است.");
        const minute = (s) =>
          Number(s.split(":")[0]) * 60 + Number(s.split(":")[1]);
        windows.push({
          days: [0, 1, 2, 3, 4, 5, 6],
          start_minute: minute(v.maintenance_start),
          end_minute: minute(v.maintenance_end),
        });
      }
      await queue(target.host_id, {
        action: "schedule",
        name: target.name,
        schedule: {
          id: uid(),
          name: target.name,
          action: v.action,
          minutes: Number(v.minutes),
          enabled: true,
          timezone: v.timezone,
          failure_threshold: 3,
          cooldown_minutes: 10,
          max_per_hour: 3,
          maintenance: false,
          maintenance_windows: windows,
        },
      });
      close();
      location.hash = "operations";
      render();
    },
    "نصب timer روی میزبان",
  );
}
function wizard() {
  const hs = hosts.filter((h) => h.kind === "iran" && h.inventory);
  if (!hs.length) {
    modal("افزودن تانل", "ابتدا یک ایران را با SSH متصل و شناسایی کنید.");
    return;
  }
  form(
    "۱ · انتخاب ایران و مشخصات تانل",
    select(
      "host",
      "سرور ایران",
      hs.map((h) => [h.id, h.name]),
    ) +
      field(
        "name",
        "نام تانل",
        "",
        "text",
        'required pattern="[a-z][a-z0-9_-]{0,9}" maxlength="10" dir="ltr"',
      ) +
      field("local", "IPv4 واقعی ایران", "", "text", 'required dir="ltr"') +
      field("remote", "IPv4 واقعی خارج", "", "text", 'required dir="ltr"') +
      field("network", "شبکهٔ خصوصی /30", "", "text", 'required dir="ltr"') +
      field("public", "پورت ورودی", "", "text", 'required dir="ltr"') +
      field("destination", "پورت مقصد", "", "text", 'required dir="ltr"') +
      select("protocol", "پروتکل", [
        ["tcp", "TCP"],
        ["udp", "UDP"],
      ]) +
      field("mtu", "MTU", "1476", "number", 'required min="576" max="1476"'),
    async (v) => {
      const config = {
        role: "entry",
        local: v.local,
        remote: v.remote,
        interface: "gf-" + v.name,
        network: v.network,
        mtu: Number(v.mtu),
        forwards: [
          {
            protocol: v.protocol,
            public: v.public,
            destination: v.destination,
          },
        ],
      };
      const plan = await queue(v.host, {
        action: "plan",
        name: v.name,
        config,
      });
      $("#form-error").textContent = "در حال بررسی فقط‌خواندنی روی ایران…";
      const o = await waitOperation(plan);
      if (o.state !== "succeeded") throw new Error(o.result?.error || o.state);
      const peer = {
        ...config,
        role: "exit",
        local: v.remote,
        remote: v.local,
        forwards: [
          {
            protocol: v.protocol,
            public: v.destination,
            destination: v.destination,
          },
        ],
      };
      const request = {
        version: 1,
        id: uid(),
        action: "create",
        name: v.name,
        config: peer,
      };
      const encoded = btoa(JSON.stringify(request));
      const command = `printf '%s' '${encoded}' | base64 -d | sudo /usr/local/bin/greflow-agent dispatch`;
      modal(
        "۲ · راه‌اندازی خارج و اعمال ایران",
        `<div class="notice">helper همین بسته را طبق راهنمای نصب روی خارج نصب کنید؛ سپس این تک‌دستور را اجرا کنید. این دستور حاوی رمز یا توکن نیست و تنها پیکربندی مشخص‌شده را اعمال می‌کند. GRE رمزنگاری ندارد.</div><pre class="code-box" id="peer-command">${esc(command)}</pre>${btn("کپی تک‌دستور", "copy-peer")}<p>پس از بررسی خروجی واقعی خارج، اعمال ایران را تأیید کنید. تست بعدی TCP معادل تأیید کلاینت نیست.</p><pre class="code-box">${esc(JSON.stringify(config, null, 2))}</pre>`,
        btn("اعمال روی ایران", "apply-wizard", "", true),
      );
      $("#modal-root").querySelector('[data-action="apply-wizard"]').onclick =
        async (event) => {
          event.stopPropagation();
          event.currentTarget.disabled = true;
          try {
            await queue(v.host, { action: "create", name: v.name, config });
            close();
            location.hash = "operations";
            render();
          } catch (e) {
            $("#form-error").textContent = e.message;
          }
        };
    },
    "بررسی plan روی ایران",
  );
}
async function action(name, id, el) {
  if (name === "reveal-password") {
    const input = document.getElementById(id),
      show = input.type === "password";
    input.type = show ? "text" : "password";
    el.textContent = show ? "پنهان" : "نمایش";
    el.setAttribute("aria-pressed", String(show));
    return;
  }
  if (name === "save-ssh-host") return;
  if (name === "apply-wizard") return;
  if (name === "close") {
    close();
    return;
  }
  if (name === "logout") {
    await api("auth/logout", {});
    user = null;
    stream?.close();
    login(false);
    return;
  }
  if (name === "add-host") {
    addHost();
    return;
  }
  if (name === "copy-ip") {
    await navigator.clipboard.writeText(id);
    toast("IP خارج کپی شد.");
    return;
  }
  if (["server_info", "server_logs"].includes(name)) {
    const operation = await queue(id, { action: name, name: "" });
    showOperation(operation);
    return;
  }
  if (name === "server-reboot") {
    const host = hosts.find((h) => h.id === id);
    form(
      "ری‌استارت خود سرور",
      `<p class="notice full">تمام سرویس‌ها و تانل‌های این سرور تا زمان بالا آمدن دوباره قطع می‌شوند.</p><p class="full">برای تأیید، آدرس <bdi class="mono">${esc(host.hostname)}</bdi> را وارد کنید.</p>` +
        field(
          "confirm",
          "آدرس سرور",
          "",
          "text",
          'required dir="ltr" autocomplete="off"',
        ),
      async (v) => {
        if (v.confirm.trim() !== host.hostname)
          throw new Error("آدرس سرور مطابقت ندارد.");
        const operation = await queue(id, {
          action: "server_reboot",
          name: "",
        });
        showOperation(operation);
      },
      "تأیید ری‌استارت سرور",
    );
    return;
  }
  if (name === "prepare") {
    const operation = await queue(id, { action: "prepare", name: "" });
    showOperation(operation);
    return;
  }
  if (name === "discover") {
    await queue(id, { action: "discover", name: "" });
    return;
  }
  if (name === "host") {
    location.hash = "tunnels?host=" + id;
    return;
  }
  if (["cards", "table"].includes(name)) {
    routeParam("view", name);
    render();
    return;
  }
  if (name === "next" || name === "prev") {
    routeParam(
      "p",
      String((Number(params().get("p")) || 1) + (name === "next" ? 1 : -1)),
    );
    render();
    return;
  }
  if (name === "detail") {
    detail(id);
    return;
  }
  if (name === "probe") {
    const t = getTunnel(id);
    await queue(t.host_id, { action: "probe", name: t.name });
    return;
  }
  if (["start", "stop", "restart", "remove", "verify"].includes(name)) {
    confirmAction(name, getTunnel(id));
    return;
  }
  if (name === "mappings") {
    mappings(getTunnel(id));
    return;
  }
  if (name === "transfer") {
    transfer(id);
    return;
  }
  if (name === "schedule") {
    schedule(getTunnel(id));
    return;
  }
  if (
    ["toggle-schedule", "delete-schedule", "maintenance-schedule"].includes(
      name,
    )
  ) {
    const s = schedules.find((v) => v.schedule.id === id);
    await queue(s.host_id, {
      action: name === "delete-schedule" ? "unschedule" : "schedule",
      name: s.schedule.name,
      schedule: {
        ...s.schedule,
        enabled:
          name === "toggle-schedule" ? !s.schedule.enabled : s.schedule.enabled,
        maintenance:
          name === "maintenance-schedule"
            ? !s.schedule.maintenance
            : s.schedule.maintenance,
      },
    });
    return;
  }
  if (name === "logs") {
    const t = getTunnel(id),
      op = await queue(t.host_id, { action: "logs", name: t.name });
    const o = await waitOperation(op);
    modal(
      "لاگ واقعی میزبان",
      `<pre class="code-box" id="real-log">${esc(o.result?.logs || o.result?.error || "لاگی ثبت نشده است.")}</pre>${btn("کپی لاگ", "copy-log")}`,
    );
    return;
  }
  if (name === "copy-log" || name === "copy-peer") {
    await navigator.clipboard.writeText(
      $(
        name === "copy-log" ? "#real-log" : "#peer-command",
      ).textContent.replace(/[\u202A-\u202E\u2066-\u2069]/g, ""),
    );
    toast("کپی شد.");
    return;
  }
  if (name === "wizard") {
    wizard();
    return;
  }
  if (name === "operation") {
    showOperation(id);
    return;
  }
  if (name === "download-report") {
    const operation = operations.find((o) => o.id === id);
    if (!operation) return;
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(operation, null, 2)], {
        type: "application/json",
      }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = `GreFlow-report-${id}.json`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    return;
  }
  if (name === "preferences") {
    const tz = $("#f-timezone").value;
    new Intl.DateTimeFormat("fa-IR", { timeZone: tz });
    localStorage.setItem("greflow-timezone", tz);
    localStorage.setItem("greflow-theme", $("#f-theme").value);
    theme();
    toast("تنظیمات نمایش ذخیره شد.");
    return;
  }
  if (name === "revoke") {
    await api("auth/revoke", {});
    user = null;
    stream?.close();
    login(false);
    return;
  }
  if (name === "schedule-runs") {
    const record = schedules.find((v) => v.schedule.id === id);
    const o = hosts
      .find((h) => h.id === record?.host_id)
      ?.inventory?.schedules?.find((x) => x.schedule.id === id);
    modal(
      "نتایج اجرا روی میزبان",
      `<p>آخرین خواندن: ${o ? time(o.observed_at) : "هنوز خوانده نشده"}</p><p class="help">برای دریافت اجرای تازه، وضعیت میزبان را دوباره بخوانید. حداکثر ۵۰ نتیجهٔ اخیر از فایل واقعی میزبان نمایش داده می‌شود.</p>` +
        ((o?.runs || [])
          .slice()
          .reverse()
          .map(
            (r) =>
              `<div class="list-row"><div><b>${time(r.at)}</b><pre class="code-box">${esc(r.detail || "")}</pre></div>${badge(r.state)}</div>`,
          )
          .join("") || "<p>نتیجه‌ای دریافت نشده است.</p>"),
    );
    return;
  }
  if (name === "sessions") {
    const sessions = await api("auth/sessions");
    modal(
      "نشست‌های من",
      sessions
        .map(
          (s) =>
            `<div class="list-row"><div><b>${s.current ? "نشست فعلی" : "نشست دیگر"}</b><p>انقضا: ${time(s.expires)}</p></div>${btn("لغو نشست", "revoke-session", s.id)}</div>`,
        )
        .join(""),
    );
    return;
  }
  if (name === "revoke-session") {
    await api("auth/sessions", { id });
    close();
    try {
      user = await api("session");
      toast("نشست لغو شد.");
    } catch {
      user = null;
      stream?.close();
      login(false);
    }
    return;
  }
  if (name === "password") {
    form(
      "تغییر رمز عبور",
      field(
        "current",
        "رمز فعلی",
        "",
        "password",
        'required autocomplete="current-password"',
      ) +
        field(
          "password",
          "رمز جدید",
          "",
          "password",
          'required minlength="8" autocomplete="new-password"',
        ) +
        '<p class="help full">پس از تغییر رمز، تمام نشست‌های شما خارج می‌شوند.</p>',
      async (v) => {
        await api("auth/password", v);
        close();
        user = null;
        stream?.close();
        login(false);
      },
      "تغییر رمز و خروج از نشست‌ها",
    );
    return;
  }
  if (name === "edit-user") {
    const account = (await api("users")).find((x) => x.id === id);
    if (!account) throw new Error("کاربر پیدا نشد.");
    form(
      `دسترسی ${account.name}`,
      select(
        "role",
        "نقش",
        [
          ["viewer", "مشاهده‌گر"],
          ["operator", "اپراتور"],
        ],
        account.role,
      ) +
        field(
          "password",
          "رمز جدید (خالی = بدون تغییر)",
          "",
          "password",
          'autocomplete="new-password"',
        ) +
        `<div class="field full"><label for="edit-scope">سرورهای مجاز</label><select id="edit-scope" name="scope" multiple>${hosts.map((h) => `<option value="${h.id}" ${(account.scope || []).includes(h.id) ? "selected" : ""}>${esc(h.name)}</option>`).join("")}</select></div><p class="help full">ذخیرهٔ دسترسی، نشست‌های قبلی این کاربر را خارج می‌کند.</p>${btn("حذف کاربر", "delete-user", account.id)}`,
      async (v, f) => {
        await api("users/manage", {
          id,
          action: "update",
          role: v.role,
          password: v.password,
          scope: [...f.querySelector("[name=scope]").selectedOptions].map(
            (x) => x.value,
          ),
        });
        close();
        toast("دسترسی کاربر به‌روز شد.");
      },
    );
    return;
  }
  if (name === "delete-user") {
    modal(
      "حذف کاربر",
      "حساب و نشست‌های این کاربر حذف می‌شود؛ سابقهٔ ممیزی حفظ می‌شود.",
      btn("تأیید حذف کاربر", "confirm-delete-user", id, true),
    );
    return;
  }
  if (name === "confirm-delete-user") {
    await api("users/manage", { id, action: "delete" });
    close();
    toast("کاربر حذف شد.");
    return;
  }
  if (name === "credentials") {
    const h = hosts.find((x) => x.id === id);
    form(
      "تغییر اطلاعات ورود",
      field("username", "کاربر SSH", h.username, "text", 'required dir="ltr"') +
        sshCredentialFields(h.auth_method || "key") +
        '<p class="help full">هویت قبلی سرور حفظ می‌شود. این کار فقط دسترسی ذخیره‌شده در پنل را تغییر می‌دهد.</p>',
      async (v) => {
        await api("servers/credentials", { id, ...v });
        close();
        await refresh();
        toast("اطلاعات ورود ذخیره شد.");
      },
    );
    return;
  }
  if (name === "users") {
    const users = await api("users");
    form(
      "کاربران و محدودهٔ دسترسی",
      `<div class="full">${users.map((u) => `<div class="list-row"><p>${esc(u.name)} · ${esc(u.role)}</p>${u.role !== "admin" ? btn("ویرایش دسترسی", "edit-user", u.id) : ""}</div>`).join("")}</div>` +
        field("name", "نام کاربری", "", "text", "required") +
        field(
          "password",
          "رمز جدید",
          "",
          "password",
          'required minlength="8" autocomplete="new-password"',
        ) +
        select("role", "نقش", [
          ["viewer", "مشاهده‌گر"],
          ["operator", "اپراتور"],
        ]) +
        `<div class="field full"><label for="user-scope">سرورهای مجاز (انتخاب چندگانه)</label><select id="user-scope" name="scope" multiple>${hosts.map((h) => `<option value="${h.id}">${esc(h.name)}</option>`).join("")}</select></div>`,
      async (v, f) => {
        await api("users", {
          name: v.name,
          password: v.password,
          role: v.role,
          scope: [...f.querySelector("[name=scope]").selectedOptions].map(
            (v) => v.value,
          ),
        });
        close();
        toast("کاربر ساخته شد.");
      },
      "افزودن کاربر",
    );
    return;
  }
  if (name === "audit") {
    const a = await api("audit");
    modal(
      "تاریخچهٔ دسترسی",
      `<pre class="code-box">${esc(JSON.stringify(a, null, 2))}</pre>`,
    );
    return;
  }
  if (name === "backup") {
    form(
      "بکاپ اختصاصی GreFlow",
      '<div class="backup-summary full"><b>تمام اطلاعات این پنل، در یک فایل</b><p>سرورها، تنظیمات، دسترسی‌ها، کانفیگ‌های شناسایی‌شده و تاریخچه. فایل فقط با رمز زیر بازیابی می‌شود.</p></div>' +
        field(
          "password",
          "رمز فایل بکاپ",
          "",
          "password",
          'required minlength="16" autocomplete="new-password"',
        ),
      async (v) => {
        const b = await api("backups", {
          password: v.password,
          scope: "full",
          include_secrets: true,
        });
        const file = {
          format: "GreFlow Backup",
          version: 1,
          created_at: new Date().toISOString(),
          payload: b.data,
        };
        const blob = new Blob([JSON.stringify(file)], {
            type: "application/vnd.greflow.backup+json",
          }),
          a = document.createElement("a");
        a.href = URL.createObjectURL(blob);
        a.download = `GreFlow-${new Date().toISOString().slice(0, 10)}.greflow`;
        a.click();
        setTimeout(() => URL.revokeObjectURL(a.href), 1000);
        close();
        toast("فایل بکاپ GreFlow دریافت شد.");
      },
      "ساخت و دریافت فایل",
    );
    return;
  }
  if (name === "restore") {
    form(
      "پیش‌نمایش بازیابی مدیریت",
      `<div class="field full"><label for="backup-file">فایل بکاپ</label><input type="file" id="backup-file" required accept=".greflow,.gfb"></div>` +
        field(
          "password",
          "رمز بکاپ",
          "",
          "password",
          'required minlength="16"',
        ),
      async (v) => {
        let data = await $("#backup-file").files[0].text();
        if (data.trim().startsWith("{")) {
          const file = JSON.parse(data);
          if (
            file.format !== "GreFlow Backup" ||
            file.version !== 1 ||
            typeof file.payload !== "string"
          )
            throw new Error("این فایل بکاپ GreFlow نیست.");
          data = file.payload;
        }
        const plan = await api("restore-plans", { data, password: v.password });
        modal(
          "بررسی اثر بازیابی",
          `<p>تعداد سرور: ${plan.hosts}</p><p>${plan.scope === "full" ? "بکاپ کامل حساب‌ها و دسترسی‌ها را جایگزین و همهٔ نشست‌ها را خارج می‌کند. پس از بازیابی با حساب موجود در بکاپ وارد شوید." : "فقط فهرست مدیریتی SSH"}</p><p>تغییری روی میزبان‌ها انجام نمی‌شود. بازیابی فقط در پنل بدون سرور و از بکاپ دارای کلید مجاز است.</p>${plan.can_restore ? "" : '<p class="form-error">این بکاپ در وضعیت فعلی قابل اعمال نیست.</p>'}`,
          plan.can_restore
            ? btn("تأیید بازیابی", "apply-restore", "", true)
            : "",
        );
        if (plan.can_restore)
          $("#modal-root").querySelector(
            "[data-action=apply-restore]",
          ).onclick = async (event) => {
            event.stopPropagation();
            event.currentTarget.disabled = true;
            try {
              const restored = await api("restore", {
                data,
                password: v.password,
                revision: plan.revision,
              });
              close();
              if (restored.logout_required) {
                user = null;
                stream?.close();
                login(false);
              } else await refresh();
            } catch (e) {
              $("#form-error").textContent = e.message;
            }
          };
      },
      "بررسی بدون تغییر",
    );
    return;
  }
  if (name === "apply-restore") return;
  if (name === "monitor") {
    const m = await api("monitoring");
    form(
      "پایش مرکزی",
      `<label class="checkbox-label full"><input type="checkbox" name="enabled" ${m.enabled ? "checked" : ""}>پایش فعال باشد</label>` +
        field(
          "minutes",
          "فاصلهٔ بررسی به دقیقه",
          m.minutes,
          "number",
          'min="1" max="60" required',
        ) +
        field(
          "retention_days",
          "نگهداری نتیجه و رخدادها به روز",
          m.retention_days,
          "number",
          'min="7" max="365" required',
        ) +
        `<p class="help full">پایش، discovery و probe فقط‌خواندنی انجام می‌دهد. برنامهٔ خاموش هشدار مرکزی ارسال نمی‌کند. شناسهٔ عملیات و تاریخچهٔ ممیزی باقی می‌مانند.</p>`,
      async (v) => {
        await api("monitoring", {
          enabled: !!v.enabled,
          minutes: Number(v.minutes),
          retention_days: Number(v.retention_days),
        });
        close();
      },
      "ذخیره",
    );
    return;
  }
  if (name === "enroll") {
    form(
      "افزودن عامل خروجی",
      field("name", "نام میزبان", "", "text", "required") +
        select("kind", "نوع", [
          ["iran", "ایران"],
          ["outside", "خارج"],
        ]) +
        `<p class="help full">نیازمند HTTPS عمومی و listener عامل با گواهی معتبر. localhost ویندوز از ایران قابل دسترس نیست.</p>`,
      async (v) => {
        const x = await api("enrollments", v);
        const url = new URL(x.url);
        if (url.protocol !== "https:" || /["'\\\s]/.test(x.url))
          throw new Error("نشانی عامل معتبر نیست");
        modal(
          "اتصال یک‌بارمصرف عامل",
          `<p>helper بسته را روی میزبان نصب کنید، سپس دستور زیر را اجرا و توکن را در ورودی مخفی وارد کنید.</p><pre class="code-box">sudo /usr/local/bin/greflow-agent enroll '${esc(x.url)}'</pre><p>توکن فقط همین بار نمایش داده می‌شود؛ انقضا: ${time(x.expires_at)}</p><pre class="code-box">${esc(x.token)}</pre><p>پس از enrollment:</p><pre class="code-box">sudo systemctl enable --now greflow-agent.service</pre>`,
        );
        await refresh();
      },
      "تولید توکن",
    );
    return;
  }
  if (name === "agents") {
    const agents = await api("enrollments");
    modal(
      "عامل‌ها و لغو دسترسی",
      agents
        .map(
          (a) =>
            `<div class="list-row"><div><b>${esc(hosts.find((h) => h.id === a.host_id)?.name || a.host_id)}</b><p>heartbeat: ${time(a.heartbeat)}</p>${badge(a.revoked ? "failed" : "ok", a.revoked ? "لغوشده یا در انتظار enrollment" : "گواهی فعال")}</div>${btn("لغو عامل و توکن‌های آن", "revoke-agent", a.host_id)}</div>`,
        )
        .join("") || "<p>عاملی ثبت نشده است.</p>",
    );
    return;
  }
  if (name === "revoke-agent") {
    await api("enrollments", { revoke: id });
    close();
    toast("دسترسی عامل لغو شد.");
    return;
  }
  if (name === "notifications") {
    const n = await api("notifications");
    form(
      "مقصد تلگرام",
      field(
        "token",
        "توکن ربات (خالی: حفظ قبلی)",
        "",
        "password",
        'autocomplete="off"',
      ) +
        field(
          "chat_id",
          "شناسهٔ مقصد",
          n.chat_id,
          "text",
          'required dir="ltr"',
        ) +
        `<label class="checkbox-label full"><input type="checkbox" name="enabled" ${n.enabled ? "checked" : ""}>ارسال هشدارهای تغییر وضعیت و بازیابی فعال باشد</label><label class="checkbox-label full"><input type="checkbox" name="test">ارسال پیام آزمایشی به همین مقصد</label>`,
      async (v) => {
        await api("notifications", {
          token: v.token,
          chat_id: v.chat_id,
          enabled: !!v.enabled,
          test: !!v.test,
        });
        close();
        toast(v.test ? "پیام ارسال شد." : "مقصد ذخیره شد.");
      },
      "ثبت / اجرای آزمایش",
    );
    return;
  }
}
document.addEventListener("click", async (e) => {
  const el = e.target.closest("[data-action]");
  if (!el || el.disabled) return;
  e.preventDefault();
  el.disabled = true;
  try {
    await action(el.dataset.action, el.dataset.id, el);
  } catch (err) {
    if ($("#form-error")) $("#form-error").textContent = err.message;
    else toast(err.message, "error");
  } finally {
    if (el.isConnected) el.disabled = false;
  }
});
document.addEventListener("input", (e) => {
  if (e.target.id === "search") {
    routeParam("q", e.target.value);
    routeParam("p", "1");
    renderResults();
  }
});
document.addEventListener("change", async (e) => {
  if (e.target.id === "f-auth_method") {
    toggleSSHMethod();
    return;
  }
  if (e.target.id === "ssh-key-file") {
    const f = e.target.files[0];
    if (f) {
      if (f.size > 65536) {
        $("#form-error").textContent = "فایل کلید بیش از حد بزرگ است.";
        return;
      }
      $("#private-key").value = await f.text();
    }
    return;
  }
  if (e.target.id === "host-filter") {
    routeParam("host", e.target.value);
    routeParam("p", "1");
    render();
  }
  if (e.target.id === "state-filter") {
    routeParam("status", e.target.value);
    routeParam("p", "1");
    renderResults();
  }
});
function theme() {
  const t = localStorage.getItem("greflow-theme") || "system";
  document.documentElement.dataset.theme =
    t === "system"
      ? matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : t;
}
theme();
matchMedia("(prefers-color-scheme: dark)").addEventListener("change", theme);
$("#theme-toggle").innerHTML = icon("moon");
$("#theme-toggle").onclick = () => {
  localStorage.setItem(
    "greflow-theme",
    document.documentElement.dataset.theme === "dark" ? "light" : "dark",
  );
  theme();
};
$("#menu-toggle").innerHTML = icon("menu");
$("#menu-toggle").onclick = () => {
  const open = $("#sidebar").classList.toggle("open");
  $("#menu-toggle").setAttribute("aria-expanded", String(open));
};
window.addEventListener("hashchange", () => {
  close();
  $("#sidebar").classList.remove("open");
  $("#menu-toggle").setAttribute("aria-expanded", "false");
  render();
});
async function start() {
  const status = await api("status");
  if (status.bootstrap_required) {
    login(true);
    return;
  }
  try {
    user = await api("session");
  } catch {
    login(false);
    return;
  }
  $("#auth").hidden = true;
  $("#auth").innerHTML = "";
  $("#shell").hidden = false;
  capabilities = await api("capabilities");
  await refresh();
  stream?.close();
  stream = new EventSource("/api/v1/events");
  stream.onmessage = () => refresh();
  stream.onerror = () =>
    notice("ارتباط رخدادها قطع شده؛ تلاش برای اتصال مجدد ادامه دارد.");
  stream.onopen = () => notice("");
}
setInterval(() => {
  if (user) refresh();
}, 15000);
start().catch((e) => {
  $("#auth").innerHTML =
    `<main class="auth-card"><h1>دسترسی به پنل ممکن نیست</h1><p role="alert">${esc(e.message)}</p><button class="btn" id="retry">تلاش مجدد</button></main>`;
  $("#retry").onclick = () => location.reload();
});

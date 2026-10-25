/* GreFlow Console design reference. All actions are LOCAL SIMULATIONS. */
(() => {
  "use strict";
  const D = window.GreFlowDomain;
  const $ = (s, el = document) => el.querySelector(s);
  const esc = (s) =>
    String(s ?? "").replace(
      /[&<>"']/g,
      (c) =>
        ({
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#39;",
        })[c],
    );
  const fa = (n) => new Intl.NumberFormat("fa-IR").format(n);
  const paths = {
    overview:
      '<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>',
    servers:
      '<rect x="3" y="3" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2"/><path d="M7 6h.01M7 17h.01M11 6h6M11 17h6"/>',
    tunnels:
      '<path d="M10 13a5 5 0 007 0l3-3a5 5 0 00-7-7l-2 2M14 11a5 5 0 00-7 0l-3 3a5 5 0 007 7l2-2"/>',
    ports: '<path d="M3 7h18l-4-4M21 17H3l4 4"/>',
    operations: '<circle cx="12" cy="12" r="9"/><path d="M10 8l6 4-6 4z"/>',
    schedules:
      '<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M16 3v4M8 3v4M3 11h18M8 15h3M8 18h6"/>',
    settings:
      '<path d="M9 3h6l1 4 4 1v8l-4 1-1 4H9l-1-4-4-1V8l4-1z"/><circle cx="12" cy="12" r="3"/>',
    plus: '<path d="M12 4v16M4 12h16"/>',
    search: '<circle cx="10" cy="10" r="6"/><path d="M15 15l6 6"/>',
    check: '<path d="M5 12l4 4L19 6"/>',
    close: '<path d="M6 6l12 12M6 18L18 6"/>',
    warning: '<path d="M12 3L2 21h20zM12 9v5M12 17v1"/>',
    clock: '<circle cx="12" cy="12" r="9"/><path d="M12 6v6l4 2"/>',
    more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
    moon: '<path d="M21 13a9 9 0 01-10-10A9 9 0 1021 13z"/>',
    sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1 1M18 18l1 1M5 19l1-1M18 6l1-1"/>',
    bell: '<path d="M18 8a6 6 0 00-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/>',
    menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
    restart: '<path d="M3 10a9 9 0 119 11M3 3v7h7"/>',
    copy: '<rect x="8" y="8" width="13" height="13" rx="2"/><path d="M16 5V3H3v13h2"/>',
    lock: '<rect x="4" y="10" width="16" height="11" rx="2"/><path d="M7 10V7a5 5 0 0110 0v3M12 14v3"/>',
    download: '<path d="M12 3v12l-4-4M12 15l4-4M3 16v5h18v-5"/>',
    stop: '<rect x="5" y="5" width="14" height="14" rx="2"/>',
    log: '<path d="M5 3h10l4 4v14H5zM9 11h6M9 15h6M9 18h4"/>',
  };
  const icon = (name) =>
    `<svg viewBox="0 0 24 24" aria-hidden="true">${paths[name] || paths.tunnels}</svg>`;
  const statusLabels = {
    healthy: "سالم",
    degraded: "اختلال پینگ",
    down: "قطع",
    unknown: "بررسی نشده",
    stale: "اطلاعات قدیمی",
    stopped: "متوقف",
  };
  const statusColor = (s) =>
    s === "healthy"
      ? "green"
      : s === "down"
        ? "red"
        : ["degraded", "stale"].includes(s)
          ? "amber"
          : "";
  const badge = (s, label) =>
    `<span class="chip ${statusColor(s)}">${icon(s === "healthy" ? "check" : s === "down" ? "close" : "warning")}${esc(label || statusLabels[s] || s)}</span>`;
  const initialServers = [
    {
      id: "iran1",
      name: "ایران اصلی",
      ip: "94.182.151.153",
      mode: "عامل",
      online: true,
      cpu: 12,
      ram: 7,
      disk: 42,
    },
    {
      id: "iran2",
      name: "ایران پشتیبان",
      ip: "94.183.160.135",
      mode: "SSH",
      online: false,
      cpu: null,
      ram: null,
      disk: null,
    },
  ];
  const initialTunnels = [
    [
      "default",
      "93.114.98.17",
      "10.77.0.0/30",
      "healthy",
      38,
      [
        [443, 443],
        [3212, 443],
      ],
      "950 MB",
      "126 MB",
    ],
    [
      "exit2",
      "91.107.165.94",
      "10.77.0.4/30",
      "healthy",
      71,
      [[444, 443]],
      "515 MB",
      "70 MB",
    ],
    [
      "exit3",
      "163.5.214.192",
      "10.77.0.8/30",
      "healthy",
      82,
      [[8443, 443]],
      "12 MB",
      "8 MB",
    ],
    [
      "exit4",
      "82.39.154.201",
      "10.77.0.12/30",
      "healthy",
      88,
      [[446, 443]],
      "55 MB",
      "29 MB",
    ],
    [
      "exit5",
      "104.234.138.231",
      "10.77.0.16/30",
      "healthy",
      84,
      [[447, 443]],
      "1.1 GB",
      "42 MB",
    ],
    [
      "exit6",
      "155.117.5.131",
      "10.77.0.20/30",
      "down",
      null,
      [[8444, 443]],
      "0 B",
      "25 KB",
    ],
    [
      "exit7",
      "212.74.39.123",
      "10.77.0.24/30",
      "degraded",
      null,
      [[8445, 443]],
      "222 MB",
      "24 MB",
    ],
    [
      "exit8",
      "57.128.107.102",
      "10.77.0.28/30",
      "healthy",
      91,
      [[8446, 443]],
      "218 MB",
      "39 MB",
    ],
    [
      "exit9",
      "91.107.244.228",
      "10.77.0.32/30",
      "healthy",
      70,
      [[2020, 2020]],
      "4.4 GB",
      "724 MB",
    ],
    [
      "exit10",
      "65.109.180.232",
      "10.77.0.36/30",
      "down",
      null,
      [],
      "0 B",
      "252 B",
    ],
    [
      "exit11",
      "66.92.161.146",
      "10.77.0.40/30",
      "healthy",
      171,
      [[2021, 2021]],
      "980 MB",
      "210 MB",
    ],
    [
      "exit12",
      "65.109.208.154",
      "10.77.0.44/30",
      "unknown",
      null,
      [[2022, 2022]],
      "—",
      "—",
    ],
  ].map(([name, remote, network, status, latency, ports, rx, tx]) => ({
    id: name,
    name,
    remote,
    network,
    status,
    latency,
    rx,
    tx,
    serverId: "iran1",
    method: "gre",
    forwards: ports.map(([p, d]) => ({
      protocol: "tcp",
      public: p,
      destination: d,
    })),
    mtu: 1476,
    checkedAt: null,
    feedback: "",
  }));
  let state = {
    servers: structuredClone(initialServers),
    tunnels: structuredClone(initialTunnels),
    serverId: "iran1",
    view: "cards",
    query: "",
    filter: "all",
    operations: [],
    schedules: [],
    page: "overview",
  };
  let timer;
  let dialogReturn;
  const busy = new Set();
  const pendingTimers = new Set();
  const navItems = [
    ["overview", "نمای کلی"],
    ["servers", "سرورها"],
    ["tunnels", "تانل‌ها"],
    ["ports", "مسیر پورت‌ها"],
    ["operations", "عملیات"],
    ["schedules", "زمان‌بندی"],
    ["settings", "تنظیمات"],
  ];
  const currentServer = () =>
    state.servers.find((s) => s.id === state.serverId);
  const serverTunnels = () =>
    state.tunnels.filter((t) => t.serverId === state.serverId);
  function toast(message) {
    $("#toast").textContent = message;
    $("#toast").classList.add("visible");
    clearTimeout(timer);
    timer = setTimeout(() => $("#toast").classList.remove("visible"), 4000);
  }
  function addOperation(
    label,
    result = "موفق",
    detail = "تغییر فقط در حافظهٔ پیش‌نمایش اعمال شد.",
  ) {
    state.operations.unshift({
      id: Date.now() + Math.random(),
      label,
      result,
      detail,
      time: new Date().toLocaleTimeString("fa-IR", {
        hour: "2-digit",
        minute: "2-digit",
      }),
    });
  }
  function heading(title, subtitle, action = "") {
    return `<div class="page-heading"><div><h1>${title}</h1><div class="subtitle">${subtitle}</div></div><div class="actions">${action}</div></div>`;
  }
  function serverPicker() {
    return `<select id="server-picker" aria-label="انتخاب ایران">${state.servers.map((s) => `<option value="${esc(s.id)}" ${s.id === state.serverId ? "selected" : ""}>${esc(s.name)} · ${esc(s.ip)}</option>`).join("")}</select>`;
  }
  function button(label, action, id = "", primary = false, extra = "") {
    return `<button class="btn ${primary ? "primary" : ""}" data-action="${action}" ${id ? `data-id="${esc(id)}"` : ""} ${extra}>${label}</button>`;
  }
  function spark(t) {
    return `<svg class="spark" viewBox="0 0 240 40" preserveAspectRatio="none" aria-label="نمودار نمونهٔ ترافیک" role="img"><path d="${t.status === "down" ? "M0 33H240" : "M0 29L12 24L24 27L36 15L48 20L60 11L72 22L84 18L96 25L108 20L120 23L132 10L144 18L156 13L168 21L180 11L192 15L204 7L216 13L228 6L240 10"}"/></svg>`;
  }
  function health(t) {
    let unknown = ["unknown", "stale", "stopped"].includes(t.status);
    return `<div class="health"><div>سرویس<span class="${unknown ? "" : "good"}">${unknown ? "نامشخص" : "✓ فعال"}</span></div><div>تانل GRE<span class="${unknown ? "" : t.status === "down" ? "bad" : t.status === "degraded" ? "warn" : "good"}">${unknown ? "نامشخص" : t.status === "down" ? "× ناموفق" : t.status === "degraded" ? "! پینگ ناموفق" : "✓ پاسخ‌گو"}</span></div><div>پورت مقصد<span class="${unknown || !t.forwards.length ? "" : t.status === "down" ? "bad" : "good"}">${!t.forwards.length ? "بدون پورت" : unknown ? "نامشخص" : t.status === "down" ? "× ناموفق" : "✓ TCP موفق"}</span></div></div>`;
  }
  function card(t) {
    const isBusy = busy.has(t.id);
    return `<article class="tunnel-card" data-card="${esc(t.id)}"><div class="card-head"><div class="name-group"><strong>${esc(t.name)}</strong><span class="chip">GRE</span></div>${badge(t.status)}</div><div class="route"><div><small>آی‌پی خارج</small><b class="mono">${esc(t.remote)}</b></div><div><small>پورت ایران → خارج</small><b class="mono">${t.forwards.length ? t.forwards.map((f) => `${f.public} → ${f.destination}`).join(" · ") : "—"}</b></div></div>${health(t)}${t.status === "down" ? '<div class="inline-error">پاسخی از مقصد دریافت نشد؛ علت نیاز به بررسی دارد.</div>' : ""}<div class="traffic">${spark(t)}<small dir="ltr">RX: ${esc(t.rx)}<br>TX: ${esc(t.tx)}</small></div><div class="card-meta"><span class="${t.latency ? "good" : "muted"}">${t.latency ? `${t.latency} ms` : "تأخیر نامشخص"}</span><span>${t.checkedAt ? "آخرین تست نمایشی: " + t.checkedAt : "دادهٔ نمونه · هنوز بررسی نشده"}</span></div>${t.feedback ? `<div class="card-feedback" role="status">${esc(t.feedback)}</div>` : ""}<div class="card-footer">${button((isBusy ? '<span class="loading"></span>' : icon("tunnels")) + " بررسی اتصال", "test", t.id, false, isBusy ? 'disabled title="بررسی این تانل در حال اجراست"' : "")}${button("جزئیات", "detail", t.id)}</div></article>`;
  }
  function summary() {
    const ts = serverTunnels();
    return `<div class="stats">${[
      [ts.length, "مجموع تانل‌ها", "tunnels"],
      [
        ts.filter((t) => t.status === "healthy").length,
        "پاسخ‌گو در دادهٔ نمونه",
        "check",
      ],
      [
        ts.filter((t) => ["down", "degraded", "stale"].includes(t.status))
          .length,
        "نیازمند بررسی",
        "warning",
      ],
      [ts.filter((t) => !t.forwards.length).length, "بدون پورت", "ports"],
    ]
      .map(
        ([n, l, i]) =>
          `<div class="stat"><span class="stat-icon">${icon(i)}</span><div><strong>${fa(n)}</strong><small>${l}</small></div></div>`,
      )
      .join("")}</div>`;
  }
  function empty(title, description, action = "") {
    return `<div class="empty">${icon("tunnels")}<h3>${title}</h3><p>${description}</p>${action}</div>`;
  }
  function overview() {
    const issues = serverTunnels().filter((t) =>
      ["down", "degraded"].includes(t.status),
    );
    return (
      heading(
        "یک نگاه به ارتباط‌ها",
        "وضعیت نمونهٔ زیرساخت شما؛ بدون اتصال زنده",
        button(icon("plus") + " افزودن ایران", "add-server", "", true),
      ) +
      summary() +
      `<div class="alert-box">${icon("warning")}<div><b>${fa(issues.length)} تانل نیاز به بررسی دارند</b><p>وضعیت سرویس، پاسخ GRE و دسترسی مقصد را جدا بررسی کنید.</p></div><a class="btn small" href="#tunnels">مشاهدهٔ تانل‌ها</a></div><div class="section-title"><h2>سرورهای ایران</h2><a class="mini-link" href="#servers">همهٔ سرورها</a></div><div class="server-grid">${state.servers.map(serverCard).join("")}</div><div class="section-title"><h2>آخرین عملیات</h2><a class="mini-link" href="#operations">تاریخچهٔ کامل</a></div><div class="panel">${operationList(3)}</div>`
    );
  }
  function serverCard(s) {
    return `<article class="server-card"><div class="card-head"><h3>${esc(s.name)}</h3>${badge(s.online ? "healthy" : "stale", s.online ? "عامل متصل" : "ارتباط مدیریتی نامشخص")}</div><div class="ip mono">${esc(s.ip)}</div><small>روش دسترسی: ${esc(s.mode)} · ${s.online ? "دادهٔ نمونه" : "آخرین داده معتبر در دسترس نیست"}</small><div class="metrics">${[
      ["CPU", s.cpu],
      ["RAM", s.ram],
      ["دیسک", s.disk],
    ]
      .map(
        ([k, v]) =>
          `<div class="metric"><small>${k}</small><b>${v === null ? "—" : fa(v) + "٪"}</b><div class="bar"><span style="width:${v ?? 0}%"></span></div></div>`,
      )
      .join(
        "",
      )}</div><div class="card-footer">${button("مدیریت تانل‌ها", "open-server", s.id)}${button("جزئیات اتصال", "server-info", s.id)}</div></article>`;
  }
  function tunnelsPage() {
    return (
      heading(
        "مدیریت تانل‌ها",
        `${esc(currentServer().name)} · <span class="mono">${esc(currentServer().ip)}</span> ${badge(currentServer().online ? "healthy" : "stale", currentServer().online ? "عامل متصل" : "دادهٔ قدیمی")}`,
        button(icon("plus") + " افزودن تانل", "add-tunnel", "", true),
      ) +
      summary() +
      `<div class="toolbar">${serverPicker()}<div class="search">${icon("search")}<input id="search" aria-label="جست‌وجوی تانل" placeholder="جست‌وجوی نام، آی‌پی یا پورت…" value="${esc(state.query)}"></div><select id="status-filter" aria-label="فیلتر وضعیت"><option value="all">همهٔ وضعیت‌ها</option>${Object.entries(
        statusLabels,
      )
        .map(
          ([v, l]) =>
            `<option value="${v}" ${state.filter === v ? "selected" : ""}>${l}</option>`,
        )
        .join(
          "",
        )}</select><div class="segments" role="group" aria-label="شیوهٔ نمایش">${[
        ["cards", "overview", "کارتی"],
        ["table", "log", "جدولی"],
      ]
        .map(
          ([v, i, l]) =>
            `<button data-action="view" data-id="${v}" class="${state.view === v ? "selected" : ""}" aria-pressed="${state.view === v}">${icon(i)}${l}</button>`,
        )
        .join("")}</div></div><div id="tunnel-results"></div>`
    );
  }
  function renderTunnelResults() {
    const ts = serverTunnels().filter(
      (t) =>
        (state.filter === "all" || t.status === state.filter) &&
        `${t.name} ${t.remote} ${t.forwards.map((f) => f.public + " " + f.destination).join(" ")}`.includes(
          state.query,
        ),
    );
    const container = $("#tunnel-results");
    if (!container) return;
    if (!ts.length) {
      container.innerHTML = empty(
        "تانلی پیدا نشد",
        "فیلترها را تغییر دهید یا یک تانل جدید اضافه کنید.",
        button("افزودن تانل", "add-tunnel", "", true),
      );
      return;
    }
    container.innerHTML =
      state.view === "cards"
        ? `<div class="cards">${ts.map(card).join("")}</div>`
        : `<div class="table-wrap"><table><thead><tr><th>نام</th><th>خارج</th><th>پورت‌ها</th><th>وضعیت</th><th>تأخیر</th><th>عملیات</th></tr></thead><tbody>${ts.map((t) => `<tr><td class="mono">${esc(t.name)}</td><td class="mono">${esc(t.remote)}</td><td class="mono">${t.forwards.map((f) => f.public + " → " + f.destination).join(" / ") || "—"}</td><td>${badge(t.status)}</td><td>${t.latency ? t.latency + " ms" : "—"}</td><td>${button("جزئیات", "detail", t.id)}</td></tr>`).join("")}</tbody></table></div>`;
  }
  function portsPage() {
    const rows = serverTunnels().flatMap((t) =>
      t.forwards.map((f) => ({ t, f })),
    );
    return (
      heading(
        "مسیر پورت‌ها",
        "مسیر ذخیره‌شدهٔ نمایشی؛ بررسی قانون واقعی در نسخهٔ متصل انجام می‌شود.",
        serverPicker(),
      ) +
      `<div class="notice">برای تشخیص مقصد اشتباه، IP ایران، تانل و IP خارج در یک ردیف نمایش داده می‌شوند. تست TCP معادل تأیید کانفیگ نیست.</div>` +
      (rows.length
        ? `<div class="table-wrap"><table><thead><tr><th>آی‌پی ایران</th><th>پروتکل</th><th>پورت ورودی</th><th>تانل</th><th>مقصد خارج</th><th>وضعیت</th><th>عملیات</th></tr></thead><tbody>${rows.map(({ t, f }) => `<tr><td class="mono">${esc(currentServer().ip)}</td><td>${f.protocol.toUpperCase()}</td><td class="mono">${f.public}</td><td class="mono">${esc(t.name)}</td><td class="mono">${esc(t.remote)}:${f.destination}</td><td>${badge(t.status)}</td><td><button class="btn small" data-action="transfer" data-id="${esc(t.id)}" data-port="${f.public}" data-protocol="${f.protocol}">تغییر مقصد</button></td></tr>`).join("")}</tbody></table></div>`
        : empty("پورتی ثبت نشده", "ابتدا یک تانل و پورت اضافه کنید."))
    );
  }
  function operationList(limit = 100) {
    return state.operations.length
      ? state.operations
          .slice(0, limit)
          .map(
            (o) =>
              `<div class="list-row"><span class="operation-icon">${icon("operations")}</span><div class="operation-main"><b>${esc(o.label)}</b><p>${esc(o.detail)}</p></div><span class="chip ${o.result === "موفق" ? "green" : "amber"}">${esc(o.result)}</span><small>${o.time}</small></div>`,
          )
          .join("")
      : empty(
          "هنوز عملیاتی اجرا نشده",
          "تست یا ویرایش نمایشی را اجرا کنید تا نتیجه اینجا ثبت شود.",
        );
  }
  function schedulePage() {
    return (
      heading(
        "زمان‌بندی و هشدارها",
        "برنامه‌ها در محصول نهایی روی خود ایران ذخیره و اجرا می‌شوند.",
        button(icon("plus") + " برنامهٔ جدید", "schedule", "", true),
      ) +
      `<div class="notice">برنامه‌های این پیش‌نمایش اجرا نمی‌شوند. در نسخهٔ واقعی اجرای بعدی همراه منطقهٔ زمانی و آخرین نتیجه نمایش داده خواهد شد.</div><div class="panel">${state.schedules.length ? state.schedules.map((s) => `<div class="list-row"><div><b>${esc(s.name)}</b><p>${esc(s.tunnel)} · هر ${fa(s.interval)} دقیقه · Asia/Tehran</p><span class="schedule-meta">اجرای بعدی: در نسخهٔ نمایشی زمان‌بندی نمی‌شود</span></div><div class="actions"><label class="switch-row"><input type="checkbox" data-schedule="${s.id}" ${s.enabled ? "checked" : ""}>فعال</label>${button("حذف", "delete-schedule", s.id)}</div></div>`).join("") : empty("برنامه‌ای وجود ندارد", "بررسی دوره‌ای، ری‌استارت یا بازیابی پس از خطا تعریف کنید.", button("افزودن برنامه", "schedule", "", true))}</div><div class="section-title"><h2>کانال‌های هشدار</h2></div><div class="panel"><div class="list-row"><div><b>تلگرام</b><p>قطعی، بازیابی و شکست عملیات؛ با جلوگیری از پیام تکراری</p></div><button class="btn" disabled aria-describedby="telegram-note">اتصال تلگرام</button></div><p id="telegram-note" class="compact-note">در این پیش‌نمایش اتصال خارجی وجود ندارد؛ در نسخهٔ متصل قابل تنظیم است.</p></div>`
    );
  }
  function settingsPage() {
    return (
      heading("تنظیمات پنل", "ترجیحات نمایش و مرز قابلیت‌های نسخهٔ نمایشی") +
      `<div class="settings-stack"><section class="panel"><h2>ظاهر و زبان</h2><div class="list-row"><div><b>حالت تاریک</b><p>تنها ترجیح پوسته در این مرورگر ذخیره می‌شود.</p></div><label class="switch-row"><input id="dark-setting" type="checkbox" ${document.documentElement.dataset.theme === "dark" ? "checked" : ""}>فعال</label></div><div class="list-row"><b>زبان رابط</b><span class="chip blue">فارسی · راست‌به‌چپ</span></div><div class="list-row"><b>منطقهٔ زمانی پیش‌فرض</b><span class="mono">Asia/Tehran</span></div></section><section class="panel"><h2>پشتیبان‌گیری</h2><p class="help">خروجی زیر فقط دادهٔ نمایشی است و شامل رمز یا کلید اتصال نیست.</p><div class="actions">${button(icon("download") + " خروجی JSON نمایشی", "export")}${button("بازنشانی نمونه‌ها", "reset")}</div></section><section class="panel"><h2>دسترسی و امنیت</h2><div class="list-row"><div><b>نقش‌ها، ورود دومرحله‌ای و نشست‌ها</b><p>در نسخهٔ متصل؛ پیش‌نمایش ورود واقعی ندارد.</p></div><span class="chip">در برنامهٔ پیاده‌سازی</span></div><div class="list-row"><div><b>روش‌های تانل</b><p>نسخهٔ اول: GRE؛ رابط توسعه برای روش‌های دیگر در سند فنی تعریف شده است.</p></div><span class="chip blue">GreFlow v0.2</span></div></section><section class="panel"><h2>وضعیت‌های رابط</h2><div class="state-gallery"><div class="state-tile"><span class="loading"></span> در حال دریافت<div class="skeleton"></div><div class="skeleton"></div></div><div class="state-tile">${badge("stale")}<p>آخرین داده معتبر نیست؛ وضعیت سبز قبلی نمایش داده نمی‌شود.</p></div><div class="state-tile">${badge("down", "عدم دسترسی")}<p>خطا در همان محل می‌ماند و گزینهٔ تلاش مجدد دارد.</p></div></div></section></div>`
    );
  }
  function render() {
    const hash = location.hash.slice(1) || "overview";
    state.page = navItems.some(([key]) => key === hash) ? hash : "overview";
    $("#navigation").innerHTML = navItems
      .map(
        ([key, label]) =>
          `<a class="nav-link ${state.page === key ? "active" : ""}" href="#${key}" ${state.page === key ? 'aria-current="page"' : ""}>${icon(key)}${label}</a>`,
      )
      .join("");
    $("#breadcrumb").textContent = navItems.find(
      ([key]) => key === state.page,
    )[1];
    const pages = {
      overview,
      servers: () =>
        heading(
          "سرورها",
          "ایران‌ها، دسترسی مدیریتی و شناسایی تانل‌های موجود",
          button(icon("plus") + " افزودن ایران", "add-server", "", true),
        ) +
        `<div class="server-grid">${state.servers.map(serverCard).join("")}</div>`,
      tunnels: tunnelsPage,
      ports: portsPage,
      operations: () =>
        heading(
          "تاریخچهٔ عملیات",
          "هر تغییر، نتیجه و خطا با شناسهٔ مستقل ثبت می‌شود.",
        ) + `<div class="panel">${operationList()}</div>`,
      schedules: schedulePage,
      settings: settingsPage,
    };
    $("#main").innerHTML = pages[state.page]();
    renderTunnelResults();
  }
  function modal(title, body, footer = "") {
    closeModal();
    dialogReturn = document.activeElement;
    $("#modal-root").innerHTML =
      `<dialog aria-labelledby="modal-title"><div class="modal-header"><h2 id="modal-title">${title}</h2><button class="icon-button" data-action="close-modal" aria-label="بستن پنجره">${icon("close")}</button></div><div class="modal-body">${body}</div>${footer ? `<div class="modal-footer">${footer}</div>` : ""}</dialog>`;
    const dlg = $("dialog");
    dlg.addEventListener("close", () => {
      if (dialogReturn?.isConnected) dialogReturn.focus();
    });
    dlg.showModal();
  }
  function closeModal() {
    const dlg = $("dialog");
    if (dlg) {
      dlg.close();
      dlg.remove();
    }
  }
  function detail(id) {
    const t = state.tunnels.find((t) => t.id === id);
    if (!t) return;
    modal(
      `جزئیات تانل ${esc(t.name)}`,
      `<div class="detail-hero"><div><span class="mono">${esc(t.remote)}</span><div class="tag-line"><span class="chip">GRE</span><span class="chip mono">${esc(t.network)}</span></div></div>${badge(t.status)}</div>${health(t)}<div class="traffic">${spark(t)}<b class="big-number mono">${t.latency ? t.latency + "ms" : "—"}</b></div><div class="notice">این وضعیت نمونه است. تأیید اتصال واقعی کانفیگ هنوز انجام نشده است.</div><div class="list-row"><b>پورت‌های ورودی → مقصد</b><span class="mono">${t.forwards.map((f) => f.public + " → " + f.destination).join(" / ") || "بدون پورت"}</span></div><div class="list-row"><b>MTU</b><span class="mono">${t.mtu}</span></div><div class="actions" style="margin-top:18px">${button(icon("restart") + " ری‌استارت", "confirm-restart", id)}${button(icon("stop") + (t.status === "stopped" ? " شروع" : " توقف"), t.status === "stopped" ? "start" : "confirm-stop", id)}${button("مدیریت پورت‌ها", "edit-ports", id)}${button("زمان‌بندی", "schedule", id)}${button("لاگ", "logs", id)}</div><details><summary>حذف تانل</summary><p class="help">حذف باید فقط منابع متعلق به همین تانل را هدف بگیرد.</p>${button("حذف تانل", "confirm-delete", id, false, 'class="btn danger"')}</details>`,
      button("بستن", "close-modal"),
    );
  }
  function test(id) {
    const t = state.tunnels.find((t) => t.id === id);
    if (!t || busy.has(id)) return;
    busy.add(id);
    t.feedback = "بررسی نمایشی در حال اجراست…";
    render();
    const timeout = setTimeout(() => {
      pendingTimers.delete(timeout);
      busy.delete(id);
      if (!state.tunnels.includes(t)) return;
      t.checkedAt = new Date().toLocaleTimeString("fa-IR", {
        hour: "2-digit",
        minute: "2-digit",
      });
      t.feedback = "شبیه‌سازی کامل شد؛ هیچ تست شبکه‌ای اجرا نشد.";
      addOperation(
        "بررسی " + t.name,
        t.status === "down" ? "نیازمند بررسی" : "موفق",
      );
      render();
      toast(t.feedback);
    }, 900);
    pendingTimers.add(timeout);
  }
  function confirmAction(action, id) {
    const t = state.tunnels.find((t) => t.id === id);
    const labels = { restart: "ری‌استارت", stop: "توقف", delete: "حذف" };
    modal(
      labels[action] + " تانل",
      `<p>تانل <b class="mono">${esc(t.name)}</b> روی <b class="mono">${esc(currentServer().ip)}</b></p><p class="help">پورت‌های تحت تأثیر: <span class="mono">${t.forwards.map((f) => f.public).join(", ") || "ندارد"}</span></p><div class="notice">این عملیات در پیش‌نمایش شبیه‌سازی می‌شود. در محصول واقعی ${action === "restart" ? "ممکن است اتصال‌های فعال این تانل قطع شوند." : action === "stop" ? "تانل تا شروع مجدد متوقف می‌ماند." : "تنظیمات و منابع اختصاصی همین تانل حذف می‌شوند."}</div>${action === "delete" ? `<div class="field"><label for="confirm-name">برای تأیید، نام ${esc(t.name)} را وارد کنید</label><input id="confirm-name" dir="ltr" autocomplete="off"></div>` : ""}<p id="confirm-error" class="form-error" role="alert"></p>`,
      button("انصراف", "close-modal") +
        button(labels[action] + " نمایشی", "execute-" + action, id, true),
    );
  }
  function editPorts(id) {
    const t = state.tunnels.find((t) => t.id === id);
    modal(
      "مدیریت پورت‌های " + esc(t.name),
      `<div id="existing-ports">${t.forwards.map((f) => `<div class="list-row"><b class="mono">${f.protocol.toUpperCase()} ${f.public} → ${f.destination}</b><button class="btn small danger" data-action="remove-port" data-id="${esc(id)}" data-port="${f.public}" data-protocol="${f.protocol}">حذف این فوروارد</button></div>`).join("") || '<p class="help">پورتی ثبت نشده است.</p>'}</div><form id="port-form" class="form-grid" style="margin-top:20px"><div class="field"><label for="public-port">پورت ایران</label><input id="public-port" name="public" inputmode="numeric" dir="ltr" required></div><div class="field"><label for="destination-port">پورت خارج</label><input id="destination-port" name="destination" inputmode="numeric" dir="ltr" required></div><div class="field"><label for="port-protocol">پروتکل</label><select id="port-protocol" name="protocol"><option value="tcp">TCP</option><option value="udp">UDP</option></select></div><div class="field"><label>&nbsp;</label><button class="btn primary" type="submit">افزودن پورت نمایشی</button></div></form><p class="form-error" id="port-error" role="alert"></p><p class="compact-note">حذف فوروارد، اینباند برنامهٔ مقصد را حذف نمی‌کند.</p>`,
    );
    $("#port-form").onsubmit = (e) => {
      e.preventDefault();
      const f = Object.fromEntries(new FormData(e.target));
      let error =
        !D.port(f.public) || !D.port(f.destination)
          ? "پورت‌ها باید بین ۱ تا ۶۵۵۳۵ باشند."
          : D.portConflict(state.tunnels, t.serverId, f.protocol, f.public)
            ? "پورت روی این ایران قبلاً رزرو شده است."
            : "";
      if (error) {
        $("#port-error").textContent = error;
        return;
      }
      t.forwards.push({
        protocol: f.protocol,
        public: Number(f.public),
        destination: Number(f.destination),
      });
      addOperation("افزودن پورت " + f.public + " به " + t.name);
      render();
      editPorts(id);
      toast("پورت به دادهٔ نمایشی اضافه شد.");
    };
  }
  function transfer(id, publicPort, protocol) {
    const from = state.tunnels.find((t) => t.id === id);
    const f = from.forwards.find(
      (f) => f.public === Number(publicPort) && f.protocol === protocol,
    );
    const targets = state.tunnels.filter(
      (t) => t.serverId === from.serverId && t.id !== from.id,
    );
    modal(
      "انتقال پورت " + publicPort,
      `<div class="transfer-route"><div><small>مسیر فعلی</small><b class="mono">${esc(from.remote)}:${f.destination}</b><span class="chip mono">${esc(from.name)}</span></div><div><small>ورودی ایران</small><b class="mono">${esc(currentServer().ip)}:${f.public}</b><span class="chip">${protocol.toUpperCase()}</span></div></div><form id="transfer-form"><div class="form-grid"><div class="field"><label for="target-tunnel">تانل مقصد</label><select id="target-tunnel" name="target">${targets.map((t) => `<option value="${esc(t.id)}">${esc(t.name)} · ${esc(t.remote)} · ${statusLabels[t.status]}</option>`).join("")}</select></div><div class="field"><label for="new-dest">پورت مقصد جدید</label><input id="new-dest" name="destination" value="${f.destination}" dir="ltr" inputmode="numeric" required></div></div><div class="notice">محصول نهایی ابتدا مقصد را تست می‌کند، سپس مسیر را جابه‌جا و نتیجه را بررسی می‌کند. نشست‌های باز ممکن است تا قطع و وصل روی مسیر قبلی بمانند.</div><label class="checkbox-label"><input type="checkbox" name="simulateFailure">شبیه‌سازی شکست پس از تغییر و برگشت خودکار</label><p id="transfer-error" class="form-error" role="alert"></p><button class="btn primary" type="submit" ${targets.length ? "" : "disabled"}>تست و انتقال نمایشی</button>${targets.length ? "" : '<p class="compact-note">برای انتقال، یک تانل دیگر روی همین ایران لازم است.</p>'}</form>`,
    );
    $("#transfer-form").onsubmit = (e) => {
      e.preventDefault();
      const data = Object.fromEntries(new FormData(e.target));
      const to = state.tunnels.find((t) => t.id === data.target);
      const error = !D.port(data.destination)
        ? "پورت مقصد معتبر نیست."
        : to.status !== "healthy"
          ? "در این شبیه‌سازی مقصد آماده نیست؛ مسیر قبلی حفظ شد."
          : "";
      if (error) {
        $("#transfer-error").textContent = error;
        return;
      }
      if (data.simulateFailure) {
        addOperation(
          "برگشت انتقال پورت " + publicPort,
          "برگشت موفق",
          "شکست نمایشی؛ مسیر قبلی بدون تغییر باقی ماند.",
        );
        $("#transfer-error").textContent =
          "خطای نمایشی پس از اعمال؛ مسیر قبلی بازیابی شد.";
        render();
        return;
      }
      from.forwards = from.forwards.filter((x) => x !== f);
      to.forwards.push({ ...f, destination: Number(data.destination) });
      addOperation(
        "انتقال پورت " + publicPort,
        "موفق",
        from.name + " → " + to.name + "؛ فقط دادهٔ نمونه",
      );
      closeModal();
      render();
      toast("مسیر نمایشی به‌روز شد؛ برای نسخهٔ واقعی تست اتصال جدید لازم است.");
    };
  }
  function schedule(tunnelId = "") {
    modal(
      "برنامهٔ جدید",
      `<form id="schedule-form"><div class="form-grid"><div class="field"><label for="schedule-target">تانل</label><select id="schedule-target" name="target">${serverTunnels()
        .map(
          (t) =>
            `<option value="${esc(t.id)}" ${t.id === tunnelId ? "selected" : ""}>${esc(t.name)}</option>`,
        )
        .join(
          "",
        )}</select></div><div class="field"><label for="schedule-type">نوع عملیات</label><select id="schedule-type" name="type"><option>بررسی دوره‌ای</option><option>ری‌استارت زمان‌بندی‌شده</option><option>بازیابی پس از خرابی</option></select></div><div class="field"><label for="schedule-interval">فاصلهٔ اجرا به دقیقه</label><input id="schedule-interval" name="interval" type="number" min="1" max="10080" value="30" required></div><div class="field"><label>منطقهٔ زمانی</label><input value="Asia/Tehran" readonly dir="ltr"></div></div><details><summary>تنظیمات بازیابی و تعمیرات</summary><p class="help">سیاست پیشنهادی: ۳ شکست متوالی، فاصلهٔ ۱۰ دقیقه بین تلاش‌ها، حداکثر ۳ بازیابی در ساعت. در حالت تعمیرات اجرا متوقف می‌شود.</p></details><div class="notice">این برنامه فقط به لیست نمایشی اضافه می‌شود؛ هیچ cron یا timer ساخته نمی‌شود.</div><p id="schedule-error" class="form-error" role="alert"></p><button class="btn primary" type="submit">ثبت برنامهٔ نمایشی</button></form>`,
    );
    $("#schedule-form").onsubmit = (e) => {
      e.preventDefault();
      const data = Object.fromEntries(new FormData(e.target));
      if (!data.target) {
        $("#schedule-error").textContent =
          "ابتدا یک تانل روی ایران اضافه کنید.";
        return;
      }
      state.schedules.push({
        id: String(Date.now()),
        name: data.type,
        tunnel: data.target,
        interval: Number(data.interval),
        enabled: true,
      });
      addOperation("ثبت برنامهٔ " + data.type);
      closeModal();
      location.hash = "schedules";
      render();
      toast("برنامهٔ نمایشی ثبت شد.");
    };
  }
  let wizard;
  function wizardValues() {
    const form = $("#wizard-form");
    if (form)
      Object.assign(wizard.values, Object.fromEntries(new FormData(form)));
  }
  function drawWizard() {
    const w = wizard;
    const values = w.values;
    const stepper = `<div class="steps">${["سرور و روش", "مقصد و پورت", "راه‌اندازی خارج", "بررسی نهایی"].map((s, i) => `<div class="step ${w.step === i ? "current" : w.step > i ? "done" : ""}"><i>${w.step > i ? "✓" : fa(i + 1)}</i>${s}</div>`).join("")}</div>`;
    let body = "";
    if (w.step === 0)
      body = `<form id="wizard-form" class="form-grid"><div class="field"><label for="wizard-server">ایران</label><select id="wizard-server" name="serverId">${state.servers.map((s) => `<option value="${s.id}" ${s.id === values.serverId ? "selected" : ""}>${esc(s.name)} · ${s.ip}</option>`).join("")}</select></div><div class="field"><label for="wizard-method">روش تانل</label><select id="wizard-method"><option>GRE — GreFlow</option></select><small>روش‌های دیگر پس از پیاده‌سازی و تست به محصول اضافه می‌شوند.</small></div><div class="notice full">تانل‌های فعلی هنگام افزودن یک اتصال جدید باید حفظ شوند. رزرو نهایی پورت و شبکه در نسخهٔ واقعی روی سرور انجام می‌شود.</div></form>`;
    if (w.step === 1)
      body = `<form id="wizard-form"><div class="form-grid"><div class="field"><label for="wizard-ip">آی‌پی خارج</label><input id="wizard-ip" name="remote" dir="ltr" value="${esc(values.remote)}" placeholder="203.0.113.20" required></div><div class="field"><label for="wizard-name">نام تانل</label><input id="wizard-name" name="name" dir="ltr" value="${esc(values.name)}" required maxlength="10"></div><div class="field"><label for="wizard-public">پورت ورودی ایران</label><input id="wizard-public" name="publicPort" dir="ltr" inputmode="numeric" value="${esc(values.publicPort)}" required></div><div class="field"><label for="wizard-dest">پورت مقصد خارج</label><input id="wizard-dest" name="destinationPort" dir="ltr" inputmode="numeric" value="${esc(values.destinationPort)}" required></div><div class="field"><label for="wizard-protocol">پروتکل پورت</label><select id="wizard-protocol" name="protocol"><option value="tcp" ${values.protocol === "tcp" ? "selected" : ""}>TCP</option><option value="udp" ${values.protocol === "udp" ? "selected" : ""}>UDP</option></select></div><div class="field"><label>شبکهٔ داخلی پیشنهادی</label><input readonly dir="ltr" value="${esc(values.network)}"><small>در نسخهٔ متصل، تداخل مسیرهای هر دو سرور نیز بررسی می‌شود.</small></div></div><details><summary>تنظیمات پیشرفته</summary><div class="field"><label for="wizard-mtu">MTU</label><input id="wizard-mtu" name="mtu" type="number" min="576" max="1476" value="${values.mtu}"></div></details></form>`;
    if (w.step === 2)
      body = `<div class="help">در نسخهٔ نهایی، دستور یک‌بارمصرف مخصوص خارج تولید می‌شود. این پیش‌نمایش دستور نصب قابل اجرا تولید نمی‌کند.</div><pre class="code-box" id="install-command"># DEMO ONLY — no server installation\n# Iran: ${esc(state.servers.find((s) => s.id === values.serverId).ip)}\n# Exit: ${esc(values.remote)}\n# Tunnel: ${esc(values.name)}\n# Ports: ${esc(values.publicPort)} → ${esc(values.destinationPort)}</pre><div class="actions" style="margin-top:15px">${button(icon("copy") + " کپی متن نمونه", "copy-install")}${button("شبیه‌سازی اتصال خارج", "simulate-enroll", "", true)}</div><div class="progress-list"><div class="progress-row">${icon(w.enrolled ? "check" : "clock")} ${w.enrolled ? "اتصال خارج شبیه‌سازی شد" : "در انتظار اجرای دستور روی خارج"}</div><div class="progress-row">${icon("lock")} اعتبار توکن در محصول: یک‌بارمصرف و ۱۰ دقیقه</div></div><details><summary>خلاصهٔ تنظیمات</summary><p class="mono">${esc(values.remote)}:${values.destinationPort}<br>${esc(values.name)} · ${esc(values.network)}</p></details>`;
    if (w.step === 3)
      body = `<div class="notice">موفقیت‌های زیر صرفاً نتیجهٔ شبیه‌سازی‌اند؛ وضعیت تانل جدید پس از ثبت «بررسی نشده» خواهد بود.</div><div class="progress-list">${["دریافت اتصال و تأیید هویت", "تطبیق شبکه و پورت‌های رزروشده", "آماده‌سازی بررسی سرویس مقصد"].map((s) => `<div class="progress-row good">${icon("check")}${s}</div>`).join("")}</div><p>مسیر پیشنهادی:</p><pre class="code-box">${esc(state.servers.find((s) => s.id === values.serverId).ip)}:${values.publicPort} → ${esc(values.remote)}:${values.destinationPort}</pre><p class="compact-note">برای UDP، تست TCP اجرا نمی‌شود و تأیید برنامهٔ مقصد لازم است.</p>`;
    modal(
      "افزودن تانل جدید",
      stepper +
        body +
        '<p id="wizard-error" class="form-error" role="alert"></p>',
      button(
        w.step ? "مرحلهٔ قبل" : "انصراف",
        w.step ? "wizard-back" : "close-modal",
      ) +
        button(
          w.step === 3 ? "ثبت تانل نمایشی" : "مرحلهٔ بعد",
          "wizard-next",
          "",
          true,
          w.step === 2 && !w.enrolled
            ? 'disabled title="ابتدا اتصال خارج را شبیه‌سازی کنید"'
            : "",
        ),
    );
  }
  function addServer() {
    modal(
      "افزودن ایران",
      `<form id="server-form"><div class="form-grid"><div class="field"><label for="server-name">نام دلخواه</label><input id="server-name" name="name" placeholder="ایران دوم" required maxlength="60"></div><div class="field"><label for="server-ip">آی‌پی سرور</label><input id="server-ip" name="ip" dir="ltr" required></div><div class="field"><label for="server-mode">روش دسترسی</label><select id="server-mode" name="mode"><option>SSH</option><option>عامل</option></select></div><div class="field"><label for="ssh-port">پورت SSH</label><input id="ssh-port" type="number" name="port" value="22" min="1" max="65535" required></div></div><div class="notice">در این پیش‌نمایش رمز یا کلید وارد نکنید. اتصال و شناسایی تانل‌های موجود در محصول واقعی انجام می‌شود.</div><p id="server-error" class="form-error" role="alert"></p><button class="btn primary" type="submit">افزودن سرور نمایشی</button></form>`,
    );
    $("#server-form").onsubmit = (e) => {
      e.preventDefault();
      const v = Object.fromEntries(new FormData(e.target));
      if (!D.ipv4(v.ip) || state.servers.some((s) => s.ip === v.ip)) {
        $("#server-error").textContent = "آی‌پی معتبر و تکرارنشده وارد کنید.";
        return;
      }
      state.servers.push({
        id: "server" + Date.now(),
        name: v.name,
        ip: v.ip,
        mode: v.mode,
        online: false,
        cpu: null,
        ram: null,
        disk: null,
      });
      addOperation(
        "افزودن ایران " + v.name,
        "ثبت شد",
        "سرور نمونه؛ هنوز اتصال مدیریتی ندارد.",
      );
      closeModal();
      location.hash = "servers";
      render();
    };
  }
  async function action(name, id, el) {
    if (name === "close-modal") {
      closeModal();
      return;
    }
    if (name === "add-server") {
      addServer();
      return;
    }
    if (name === "open-server") {
      state.serverId = id;
      state.query = "";
      location.hash = "tunnels";
      render();
      return;
    }
    if (name === "server-info") {
      const s = state.servers.find((s) => s.id === id);
      modal(
        "جزئیات اتصال ایران",
        `<p>${esc(s.name)} — <span class="mono">${s.ip}</span></p><div class="notice">${s.online ? "اتصال عامل نمونه است؛ اتصال زنده برقرار نشده." : "دسترسی مدیریتی نامشخص است؛ اطلاعات سلامت نامعلوم می‌ماند."}</div><p class="help">در نسخهٔ واقعی: بررسی اثرانگشت SSH، تعویض کلید، نسخهٔ عامل و لغو دسترسی در این بخش قرار می‌گیرد.</p>`,
      );
      return;
    }
    if (name === "view") {
      state.view = id;
      render();
      return;
    }
    if (name === "test") {
      test(id);
      return;
    }
    if (name === "detail") {
      detail(id);
      return;
    }
    if (name.startsWith("confirm-")) {
      confirmAction(name.slice(8), id);
      return;
    }
    if (name.startsWith("execute-")) {
      const t = state.tunnels.find((t) => t.id === id);
      if (name === "execute-delete") {
        if ($("#confirm-name").value !== t.name) {
          $("#confirm-error").textContent = "نام تانل مطابق نیست.";
          return;
        }
        state.tunnels = state.tunnels.filter((x) => x !== t);
        state.schedules = state.schedules.filter((s) => s.tunnel !== id);
      } else if (name === "execute-stop") t.status = "stopped";
      else
        t.feedback =
          "ری‌استارت شبیه‌سازی شد؛ وضعیت شبکه دوباره باید بررسی شود.";
      if (name === "execute-restart") t.status = "unknown";
      addOperation(
        name === "execute-delete"
          ? "حذف " + t.name
          : name === "execute-stop"
            ? "توقف " + t.name
            : "ری‌استارت " + t.name,
      );
      closeModal();
      render();
      toast("عملیات فقط در پیش‌نمایش اعمال شد.");
      return;
    }
    if (name === "start") {
      const t = state.tunnels.find((t) => t.id === id);
      t.status = "unknown";
      addOperation("شروع " + t.name);
      closeModal();
      render();
      return;
    }
    if (name === "edit-ports") {
      editPorts(id);
      return;
    }
    if (name === "remove-port") {
      const t = state.tunnels.find((t) => t.id === id);
      t.forwards = t.forwards.filter(
        (f) =>
          !(
            f.public === Number(el.dataset.port) &&
            f.protocol === el.dataset.protocol
          ),
      );
      addOperation("حذف فوروارد " + el.dataset.port);
      render();
      editPorts(id);
      return;
    }
    if (name === "transfer") {
      transfer(id, el.dataset.port, el.dataset.protocol);
      return;
    }
    if (name === "schedule") {
      schedule(id);
      return;
    }
    if (name === "delete-schedule") {
      state.schedules = state.schedules.filter((s) => s.id !== id);
      render();
      toast("برنامهٔ نمایشی حذف شد.");
      return;
    }
    if (name === "logs") {
      modal(
        "لاگ تانل " + esc(id),
        `<div class="notice">لاگ نمونه؛ بدون اتصال به سرور</div><pre class="code-box">${esc(
          state.operations
            .filter((o) => o.label.includes(id))
            .map((o) => o.time + " " + o.label + " " + o.result)
            .join("\n") || "No recorded demo operations for this tunnel.",
        )}</pre>`,
      );
      return;
    }
    if (name === "add-tunnel") {
      const allocation = D.allocate(state.tunnels, state.serverId);
      wizard = {
        step: 0,
        enrolled: false,
        values: {
          serverId: state.serverId,
          ...allocation,
          remote: "",
          publicPort: "",
          destinationPort: "",
          protocol: "tcp",
          mtu: 1476,
        },
      };
      drawWizard();
      return;
    }
    if (name === "wizard-back") {
      wizardValues();
      wizard.step--;
      drawWizard();
      return;
    }
    if (name === "wizard-next") {
      wizardValues();
      if (wizard.step === 0) {
        Object.assign(
          wizard.values,
          D.allocate(state.tunnels, wizard.values.serverId),
        );
      }
      if (wizard.step === 1) {
        const error = D.validateTunnel(wizard.values, state.tunnels);
        if (error) {
          $("#wizard-error").textContent = error;
          return;
        }
      }
      if (wizard.step === 3) {
        const v = wizard.values;
        const error = D.validateTunnel(v, state.tunnels);
        if (error) {
          $("#wizard-error").textContent = error;
          return;
        }
        state.tunnels.push({
          id: v.serverId + "-" + v.name,
          ...v,
          method: "gre",
          status: "unknown",
          latency: null,
          rx: "—",
          tx: "—",
          checkedAt: null,
          forwards: [
            {
              protocol: v.protocol,
              public: Number(v.publicPort),
              destination: Number(v.destinationPort),
            },
          ],
        });
        state.serverId = v.serverId;
        addOperation(
          "افزودن تانل " + v.name,
          "ثبت شد",
          "تنظیم نمایشی؛ هیچ دستور سروری اجرا نشده است.",
        );
        closeModal();
        location.hash = "tunnels";
        render();
        toast("تانل نمایشی اضافه شد.");
        return;
      }
      wizard.step++;
      drawWizard();
      return;
    }
    if (name === "simulate-enroll") {
      wizard.enrolled = true;
      drawWizard();
      return;
    }
    if (name === "copy-install") {
      const text = $("#install-command").textContent;
      try {
        await navigator.clipboard.writeText(text);
        toast("متن نمونه کپی شد.");
      } catch {
        toast("کپی خودکار در دسترس نیست؛ متن را انتخاب و کپی کنید.");
      }
      return;
    }
    if (name === "export") {
      const blob = new Blob(
        [
          JSON.stringify(
            {
              demo: true,
              servers: state.servers,
              tunnels: state.tunnels,
              schedules: state.schedules,
            },
            null,
            2,
          ),
        ],
        { type: "application/json" },
      );
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = "greflow-demo-export.json";
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 1000);
      return;
    }
    if (name === "reset") {
      pendingTimers.forEach(clearTimeout);
      pendingTimers.clear();
      busy.clear();
      state = {
        ...state,
        servers: structuredClone(initialServers),
        tunnels: structuredClone(initialTunnels),
        serverId: "iran1",
        operations: [],
        schedules: [],
        query: "",
        filter: "all",
      };
      render();
      toast("داده‌های نمونه بازنشانی شدند.");
    }
  }
  document.addEventListener("click", (e) => {
    const el = e.target.closest("[data-action]");
    if (el && !el.disabled) action(el.dataset.action, el.dataset.id || "", el);
  });
  document.addEventListener("input", (e) => {
    if (e.target.id === "search") {
      state.query = e.target.value;
      renderTunnelResults();
    }
  });
  document.addEventListener("change", (e) => {
    if (e.target.id === "server-picker") {
      state.serverId = e.target.value;
      state.query = "";
      render();
    }
    if (e.target.id === "status-filter") {
      state.filter = e.target.value;
      renderTunnelResults();
    }
    if (e.target.id === "dark-setting") toggleTheme();
    if (e.target.dataset.schedule) {
      const s = state.schedules.find((s) => s.id === e.target.dataset.schedule);
      s.enabled = e.target.checked;
      toast("فعال‌بودن برنامهٔ نمایشی تغییر کرد.");
    }
  });
  function toggleTheme() {
    const next =
      document.documentElement.dataset.theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem("greflow-theme", next);
    } catch {}
    updateThemeButton();
    if (state.page === "settings") render();
  }
  function updateThemeButton() {
    const dark = document.documentElement.dataset.theme === "dark";
    $("#theme-toggle").innerHTML = icon(dark ? "sun" : "moon");
    $("#theme-toggle").setAttribute(
      "aria-label",
      dark ? "تغییر به حالت روشن" : "تغییر به حالت تاریک",
    );
  }
  let theme;
  try {
    theme = localStorage.getItem("greflow-theme");
  } catch {}
  document.documentElement.dataset.theme =
    theme ||
    (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  updateThemeButton();
  $("#theme-toggle").onclick = toggleTheme;
  $("#menu-toggle").innerHTML = icon("menu");
  $("#menu-toggle").onclick = () => {
    const open = $("#sidebar").classList.toggle("open");
    $("#menu-toggle").setAttribute("aria-expanded", String(open));
  };
  $("#alerts-button").innerHTML = icon("bell");
  $("#alerts-button").onclick = () => {
    state.filter = "down";
    location.hash = "tunnels";
    render();
  };
  window.addEventListener("hashchange", () => {
    closeModal();
    $("#sidebar").classList.remove("open");
    $("#menu-toggle").setAttribute("aria-expanded", "false");
    render();
    $("#main").focus();
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      $("#sidebar").classList.remove("open");
      $("#menu-toggle").setAttribute("aria-expanded", "false");
    }
  });
  render();
})();

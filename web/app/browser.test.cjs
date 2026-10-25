const { test } = require("node:test");
const assert = require("node:assert/strict");
const { spawn } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const { chromium } = require("playwright");

test("production binary: bootstrap, login, RBAC, real empty states, forms and responsive UI", async (t) => {
  const data = fs.mkdtempSync(path.join(os.tmpdir(), "greflow-browser-"));
  const binary =
    process.env.GREFLOW_CONSOLE_BINARY ||
    path.resolve("dist/greflow-console-windows-amd64.exe");
  const server = spawn(
    binary,
    [
      "--data",
      path.join(data, "data"),
      "--key-file",
      path.join(data, "key"),
      "--listen",
      "127.0.0.1:18788",
    ],
    { stdio: ["ignore", "pipe", "pipe"] },
  );
  let logs = "";
  server.stderr.on("data", (b) => (logs += b));
  t.after(() => {
    server.kill();
  });
  for (let i = 0; i < 100; i++) {
    try {
      const r = await fetch("http://127.0.0.1:18788/api/v1/status");
      if (r.ok) break;
    } catch {}
    await new Promise((r) => setTimeout(r, 100));
  }
  const browser = await chromium.launch({
    headless: true,
    channel: process.env.GREFLOW_BROWSER_CHANNEL || "chrome",
  });
  t.after(() => browser.close());
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("http://127.0.0.1:18788");
  await page.getByRole("heading", { name: "رمز پنل را تعیین کنید" }).waitFor();
  assert.equal(
    await page.getByLabel("کد راه‌اندازی", { exact: true }).count(),
    0,
  );
  await page
    .getByLabel("رمز عبور", { exact: true })
    .fill("test-only-password-2026");
  await page
    .getByRole("button", { name: "نمایش رمز عبور", exact: true })
    .click();
  assert.equal(
    await page.getByLabel("رمز عبور", { exact: true }).getAttribute("type"),
    "text",
  );
  await page
    .getByRole("button", { name: "نمایش رمز عبور", exact: true })
    .click();
  await page
    .getByLabel("تکرار رمز عبور", { exact: true })
    .fill("mismatched-password");
  await page
    .getByRole("button", { name: "شروع کار با GreFlow", exact: true })
    .click();
  await page
    .getByText("تکرار رمز با رمز عبور یکسان نیست.", { exact: true })
    .waitFor();
  await page
    .getByLabel("تکرار رمز عبور", { exact: true })
    .fill("test-only-password-2026");
  await page
    .getByRole("button", { name: "شروع کار با GreFlow", exact: true })
    .click();
  await page
    .getByRole("heading", { name: "اولین سرور را متصل کنید" })
    .waitFor();
  assert.equal(await page.locator(".tunnel-card").count(), 0);
  for (const section of [
    "servers",
    "tunnels",
    "ports",
    "operations",
    "schedules",
    "settings",
  ]) {
    await page.locator(`#navigation a[href="#${section}"]`).click();
    await page.locator("#main h1").waitFor();
  }
  await page.locator('#navigation a[href="#servers"]').click();
  await page.locator('#main [data-action="add-host"]').click();
  await page.locator("dialog").waitFor();
  assert.equal(
    await page.getByLabel("اثر انگشت SHA256 تأییدشده", { exact: true }).count(),
    0,
  );
  await page.getByLabel("پسورد SSH", { exact: true }).waitFor();
  fs.mkdirSync("test-results", { recursive: true });
  await page.screenshot({
    animations: "disabled",
    path: "test-results/ssh-password-dialog.png",
  });
  await page.setViewportSize({ width: 390, height: 950 });
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    true,
  );
  await page.screenshot({
    animations: "disabled",
    path: "test-results/ssh-dialog-mobile.png",
  });
  await page.getByLabel("روش ورود", { exact: true }).selectOption("key");
  await page.getByLabel("کلید خصوصی SSH", { exact: true }).waitFor();
  assert.equal(
    await page.getByLabel("پسورد SSH", { exact: true }).isEnabled(),
    false,
  );
  await page.getByLabel("رمز کلید (اگر دارد)", { exact: true }).waitFor();
  await page.screenshot({
    animations: "disabled",
    path: "test-results/ssh-key-dialog.png",
  });

  await page.keyboard.press("Tab");
  assert.equal(
    await page.evaluate(
      () => document.activeElement.closest("dialog") !== null,
    ),
    true,
  );
  await page.keyboard.press("Escape");
  assert.equal(await page.locator("dialog[open]").count(), 0);
  fs.mkdirSync("test-results", { recursive: true });
  for (const width of [360, 390, 768, 1280, 1440]) {
    await page.setViewportSize({ width, height: 950 });
    await page.goto("http://127.0.0.1:18788/#overview");
    await page
      .getByRole("heading", { name: "اولین سرور را متصل کنید" })
      .waitFor();
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      true,
      `overflow ${width}`,
    );
    await page.screenshot({
      animations: "disabled",
      path: `test-results/console-${width}.png`,
      fullPage: true,
    });
  }
  await page.locator("#theme-toggle").click();
  assert.equal(await page.locator("html").getAttribute("data-theme"), "dark");
  await page.screenshot({
    animations: "disabled",
    path: "test-results/console-dark.png",
    fullPage: true,
  });
  assert.equal(
    await page.evaluate(() => document.fonts.check("14px Vazirmatn")),
    true,
  );
  assert.deepEqual(
    await page.evaluate(() =>
      Object.keys(localStorage).filter((k) =>
        /token|key|password|secret|session/.test(k),
      ),
    ),
    [],
  );
  // These controls use the actual backend, before the explicit layout fixture.
  await page.goto("http://127.0.0.1:18788/#settings");
  await page.getByRole("button", { name: "نشست‌های من", exact: true }).click();
  await page.getByText("نشست فعلی", { exact: true }).waitFor();
  await page.keyboard.press("Escape");
  await page
    .getByRole("button", { name: "مدیریت کاربران", exact: true })
    .click();
  await page.getByLabel("نام کاربری", { exact: true }).fill("browser-viewer");
  await page
    .getByLabel("رمز جدید", { exact: true })
    .fill("browser-only-long-password");
  await page.getByRole("button", { name: "افزودن کاربر", exact: true }).click();
  await page.locator("dialog").waitFor({ state: "detached" });
  await page
    .getByRole("button", { name: "مدیریت کاربران", exact: true })
    .click();
  await page
    .getByRole("button", { name: "ویرایش دسترسی", exact: true })
    .click();
  await page.getByLabel("نقش", { exact: true }).selectOption("operator");
  await page.getByRole("button", { name: "ذخیره", exact: true }).click();
  await page.locator("dialog").waitFor({ state: "detached" });
  await page.getByRole("button", { name: "ساخت بکاپ", exact: true }).click();
  await page
    .getByLabel("رمز فایل بکاپ", { exact: true })
    .fill("browser-backup-password-2026");
  const downloadPromise = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "ساخت و دریافت فایل", exact: true })
    .click();
  const download = await downloadPromise;
  assert.match(download.suggestedFilename(), /^GreFlow-.*\.greflow$/);
  const backupFile = JSON.parse(fs.readFileSync(await download.path(), "utf8"));
  assert.equal(backupFile.format, "GreFlow Backup");
  assert.equal(
    Buffer.from(backupFile.payload, "base64").subarray(0, 4).toString(),
    "GFB1",
  );
  await page.locator("dialog").waitFor({ state: "detached" });
  await page
    .getByRole("button", { name: "پیش‌نمایش بازیابی", exact: true })
    .click();
  await page.locator("#backup-file").setInputFiles({
    name: "backup.greflow",
    mimeType: "application/json",
    buffer: Buffer.from(JSON.stringify(backupFile)),
  });
  await page
    .getByLabel("رمز بکاپ", { exact: true })
    .fill("browser-backup-password-2026");
  await page
    .getByRole("button", { name: "بررسی بدون تغییر", exact: true })
    .click();
  await page
    .getByRole("heading", { name: "بررسی اثر بازیابی", exact: true })
    .waitFor();
  await page.keyboard.press("Escape");
  assert.equal(await page.getByRole("button", { name: /TOTP/ }).count(), 0);
  // Explicit UI-only fixture transport. It is not compiled into the application,
  // and no remote host/configuration is created by these layout assertions.
  const now = new Date(),
    expiry = new Date(now.getTime() + 180000).toISOString();
  const tunnels = Array.from({ length: 27 }, (_, i) => ({
    name: "exit" + i,
    revision: "test-revision",
    desired: {
      role: "entry",
      local: "192.0.2.1",
      remote: "198.51.100." + (i + 1),
      interface: "gf-exit" + i,
      network: `10.77.0.${i * 4}/30`,
      mtu: 1476,
      forwards: [
        { protocol: "tcp", public: String(4000 + i), destination: "443" },
      ],
    },
    checks: [
      "service",
      "interface",
      "rules",
      "gre_peer",
      "destination:tcp:" + (4000 + i),
      "user_verified",
    ].map((kind) => ({
      kind,
      status: kind === "user_verified" ? "unknown" : "ok",
      observed_at: now.toISOString(),
      expires_at:
        i === 0 ? new Date(now.getTime() - 1000).toISOString() : expiry,
    })),
    drift: false,
    rx_bytes: 1234,
    tx_bytes: 567,
  }));
  await page.route("**/api/v1/servers", (route) =>
    route.fulfill({
      json: [
        {
          id: "test-host",
          name: "میزبان آزمایشی رابط",
          kind: "iran",
          hostname: "192.0.2.1",
          port: 22,
          addresses: ["192.0.2.1/24", "2001:db8::1/64"],
          inventory: {
            host_id: "test-host",
            observed_at: now.toISOString(),
            tunnels,
          },
          observed_at: now.toISOString(),
        },
      ],
    }),
  );
  const preparationReport = {
    id: "isolated-preparation-report",
    host_id: "test-host",
    state: "failed",
    request: { action: "prepare", name: "" },
    updated_at: now.toISOString(),
    result: {
      state: "failed",
      error: "نوشتن فایل کامل نشد",
      diagnostics: [
        {
          at: now.toISOString(),
          stage: "ssh",
          state: "succeeded",
          message: "ورود SSH تأیید شد",
        },
        {
          at: now.toISOString(),
          stage: "install",
          state: "failed",
          code: "UPLOAD_WRITE",
          exit_code: 44,
          message: "فضای دیسک و ارتباط SSH را بررسی کنید",
        },
      ],
    },
  };
  await page.route("**/api/v1/operations", (route) =>
    route.fulfill({ json: [preparationReport] }),
  );
  await page.route("**/api/v1/ip-info?**", (route) =>
    route.fulfill({
      json: {
        ip: "198.51.100.1",
        status: "ok",
        country: "Germany",
        country_code: "de",
        city: "Falkenstein",
        organization: "UI fixture network",
        asn: "AS64500",
        source: "UI fixture — not live geography",
        observed_at: now.toISOString(),
        expires_at: expiry,
      },
    }),
  );
  await page.reload();
  await page.goto("http://127.0.0.1:18788/#servers");
  await page
    .getByRole("button", { name: "مدیریت تانل‌ها", exact: true })
    .waitFor();
  await page
    .getByRole("button", { name: "گزارش اتصال و عملیات", exact: true })
    .click();
  await page
    .locator(".diagnostic-list")
    .getByText("UPLOAD_WRITE", { exact: true })
    .waitFor();
  assert.match(await page.locator(".diagnostic-list").innerText(), /44/);
  const reportDownload = page.waitForEvent("download");
  await page.getByRole("button", { name: "دانلود گزارش", exact: true }).click();
  const savedReport = await reportDownload;
  assert.equal(
    JSON.parse(fs.readFileSync(await savedReport.path(), "utf8")).result
      .diagnostics[1].code,
    "UPLOAD_WRITE",
  );
  for (const width of [360, 1440]) {
    await page.setViewportSize({ width, height: 950 });
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      true,
    );
    await page.screenshot({
      path: `test-results/preparation-report-${width}.png`,
    });
  }
  await page.keyboard.press("Escape");
  await page.locator(".server-tools summary").click();
  await page
    .getByRole("button", { name: "ری‌استارت سرور", exact: true })
    .click();
  await page.getByLabel("آدرس سرور", { exact: true }).fill("incorrect-address");
  await page
    .getByRole("button", { name: "تأیید ری‌استارت سرور", exact: true })
    .click();
  await page.getByText("آدرس سرور مطابقت ندارد.", { exact: true }).waitFor();
  await page.keyboard.press("Escape");
  await page.goto("http://127.0.0.1:18788/#tunnels");
  await page.locator(".tunnel-card").first().waitFor();
  await page
    .locator(".tunnel-card")
    .first()
    .getByAltText("پرچم آلمان")
    .waitFor();
  assert.equal(
    await page.locator(".tunnel-card h3").first().innerText(),
    "198.51.100.1",
  );
  assert.match(
    await page
      .locator(".tunnel-card")
      .first()
      .getByRole("link", { name: "Check‑Host ↗" })
      .getAttribute("href"),
    /host=198\.51\.100\.1/,
  );
  assert.equal(await page.locator(".tunnel-card").count(), 24);
  await page.locator("#search").fill("exit0");
  assert.equal(await page.locator(".tunnel-card").count(), 1);
  assert.equal(
    (await page
      .locator(".tunnel-card")
      .getByText("دادهٔ قدیمی", { exact: true })
      .count()) > 0,
    true,
  );
  await page.locator("#search").fill("");
  await page.locator('[data-action="table"]').click();
  assert.equal(await page.locator("#results tbody tr").count(), 24);
  await page.locator('[data-action="cards"]').click();
  for (const width of [360, 390, 768, 1280, 1440]) {
    await page.setViewportSize({ width, height: 950 });
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      true,
      `populated overflow ${width}`,
    );
    await page.screenshot({
      animations: "disabled",
      path: `test-results/fixture-tunnels-${width}.png`,
      fullPage: false,
    });
  }
  await page.locator('[data-action="detail"]').first().click();
  for (let i = 0; i < 20; i++) {
    await page.keyboard.press("Tab");
    assert.equal(
      await page.evaluate(() => !!document.activeElement.closest("dialog")),
      true,
    );
  }
  await page.keyboard.press("Escape");
  await page.goto("http://127.0.0.1:18788/#ports");
  await page.locator("tbody tr").first().waitFor();
  await page.setViewportSize({ width: 390, height: 950 });
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    true,
    "port table body overflow",
  );
  assert.deepEqual(errors, []);
  assert.equal(logs, "");
});

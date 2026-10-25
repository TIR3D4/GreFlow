/* Optional browser QA: npm install --no-save playwright @sparticuz/chromium
 * or use Playwright's installed Chromium without CHROMIUM_MODULE.
 * Run with the preview directory as CWD, or run this file from repository root.
 */
"use strict";
const { test } = require("node:test");
const assert = require("node:assert/strict");
const { spawn } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const { chromium } = require("playwright");
let browser, server;
test("console workflows, keyboard dialog behavior and responsive layouts", async (t) => {
  const port = 18787;
  server = spawn(process.execPath, [path.join(__dirname, "serve.cjs")], {
    env: { ...process.env, GREFLOW_PREVIEW_PORT: String(port) },
    stdio: ["ignore", "pipe", "pipe"],
  });
  t.after(async () => {
    if (browser) await browser.close();
    server.kill();
  });
  await new Promise((resolve, reject) => {
    server.stdout.once("data", resolve);
    server.once("error", reject);
    server.once("exit", (code) => reject(new Error("preview exited " + code)));
  });
  const launch = { headless: true };
  if (process.env.CHROMIUM_MODULE) {
    const imported = await import(
      require("node:url").pathToFileURL(process.env.CHROMIUM_MODULE).href
    );
    const packed = imported.default;
    launch.executablePath = await packed.executablePath();
    launch.args = packed.args;
  }
  browser = await chromium.launch(launch);
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
    colorScheme: "light",
  });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto(`http://127.0.0.1:${port}`);
  await page.evaluate(() => document.fonts.ready);
  assert.equal(
    await page.evaluate(() => document.fonts.check("14px Vazirmatn")),
    true,
  );
  assert.equal(
    await page.getByRole("heading", { name: "یک نگاه به ارتباط‌ها" }).count(),
    1,
  );
  await page.locator('#navigation a[href="#tunnels"]').click();
  assert.equal(await page.locator(".tunnel-card").count(), 12);
  await page.locator("#search").fill("exit11");
  assert.equal(await page.locator(".tunnel-card").count(), 1);
  await page.locator("#search").fill("");
  await page.locator('[data-action="view"][data-id="table"]').click();
  assert.equal(await page.locator("#tunnel-results tbody tr").count(), 12);
  await page.locator('[data-action="view"][data-id="cards"]').click();
  await page.locator('[data-action="test"][data-id="exit2"]').click();
  await page.waitForFunction(() =>
    document
      .querySelector('[data-card="exit2"] .card-feedback')
      ?.textContent.includes("شبیه‌سازی کامل"),
  );
  await page.locator('[data-action="detail"][data-id="exit2"]').click();
  assert.equal(await page.locator("dialog[open]").count(), 1);
  await page.keyboard.press("Escape");
  assert.equal(await page.locator("dialog[open]").count(), 0);
  await page.locator('[data-action="add-tunnel"]').click();
  await page.locator('[data-action="wizard-next"]').click();
  await page.locator("#wizard-ip").fill("203.0.113.77");
  await page.locator("#wizard-public").fill("2021");
  await page.locator("#wizard-dest").fill("2022");
  await page.locator('[data-action="wizard-next"]').click();
  assert.ok(
    (await page.locator("#wizard-error").textContent()).includes("رزرو"),
  );
  await page.locator("#wizard-public").fill("3030");
  await page.locator('[data-action="wizard-next"]').click();
  assert.equal(
    await page.locator('[data-action="wizard-next"]').isDisabled(),
    true,
  );
  await page.locator('[data-action="simulate-enroll"]').click();
  await page.locator('[data-action="wizard-next"]').click();
  await page.locator('[data-action="wizard-next"]').click();
  assert.equal(await page.locator(".tunnel-card").count(), 13);
  await page.locator('#navigation a[href="#ports"]').click();
  await page.locator('[data-action="transfer"][data-id="exit11"]').click();
  await page.locator("#target-tunnel").selectOption("exit9");
  await page.locator('[name="simulateFailure"]').check();
  await page.locator('#transfer-form button[type="submit"]').click();
  assert.ok(
    (await page.locator("#transfer-error").textContent()).includes("بازیابی"),
  );
  await page.locator('[name="simulateFailure"]').uncheck();
  await page.locator('#transfer-form button[type="submit"]').click();
  const mappingRow = page
    .locator("tbody tr")
    .filter({
      has: page.locator('[data-action="transfer"][data-port="2021"]'),
    });
  assert.ok((await mappingRow.textContent()).includes("91.107.244.228"));
  await page.locator('#navigation a[href="#schedules"]').click();
  await page.locator('.page-heading [data-action="schedule"]').click();
  await page.locator('#schedule-form button[type="submit"]').click();
  assert.equal(await page.locator("[data-schedule]").count(), 1);
  await page.locator('#navigation a[href="#settings"]').click();
  await page.locator("#dark-setting").check();
  assert.equal(await page.locator("html").getAttribute("data-theme"), "dark");
  await page.locator("#theme-toggle").click();
  await page.locator('#navigation a[href="#tunnels"]').click();
  const output = process.env.GREFLOW_QA_DIR;
  if (output) {
    fs.mkdirSync(output, { recursive: true });
    await page.screenshot({
      path: path.join(output, "desktop-reference.png"),
      fullPage: true,
    });
  }
  for (const width of [360, 390, 768, 1280, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    const overflow = await page.evaluate(() => ({
      width: innerWidth,
      scroll: document.documentElement.scrollWidth,
      elements: [...document.querySelectorAll("body *")]
        .filter((el) => {
          const r = el.getBoundingClientRect();
          return r.width && (r.right > innerWidth + 1 || r.left < -1);
        })
        .slice(0, 10)
        .map((el) => ({
          tag: el.tagName,
          cls: el.className,
          left: el.getBoundingClientRect().left,
          right: el.getBoundingClientRect().right,
        })),
    }));
    assert.equal(
      overflow.scroll > width,
      false,
      "body overflow at " + width + " " + JSON.stringify(overflow),
    );
  }
  await page.setViewportSize({ width: 390, height: 844 });
  if (output)
    await page.screenshot({
      path: path.join(output, "mobile-reference.png"),
      fullPage: true,
    });
  await page.locator("#menu-toggle").click();
  await page.locator('#navigation a[href="#servers"]').click();
  await page.locator('[data-action="add-server"]').click();
  await page.locator("#server-name").fill("ایران تست");
  await page.locator("#server-ip").fill("203.0.113.88");
  await page.locator('#server-form button[type="submit"]').click();
  assert.equal(await page.locator(".server-card").count(), 3);
  assert.deepEqual(errors, []);
});

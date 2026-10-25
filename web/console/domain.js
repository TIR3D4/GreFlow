/* Shared, dependency-free validators. This file never invokes the network or shell. */
(function (root) {
  "use strict";
  const ipv4 = (value) =>
    typeof value === "string" &&
    /^(\d{1,3}\.){3}\d{1,3}$/.test(value) &&
    value.split(".").every((n) => String(Number(n)) === n && Number(n) <= 255);
  const port = (value) =>
    /^\d+$/.test(String(value)) && Number(value) >= 1 && Number(value) <= 65535;
  const portConflict = (tunnels, serverId, protocol, publicPort, exceptId) =>
    tunnels.some(
      (t) =>
        t.id !== exceptId &&
        t.serverId === serverId &&
        t.forwards.some(
          (f) =>
            f.protocol === protocol && Number(f.public) === Number(publicPort),
        ),
    );
  function allocate(tunnels, serverId) {
    const usedNames = new Set(
      tunnels.filter((t) => t.serverId === serverId).map((t) => t.name),
    );
    const usedNetworks = new Set(
      tunnels.filter((t) => t.serverId === serverId).map((t) => t.network),
    );
    let index = 2;
    while (usedNames.has("exit" + index)) index++;
    // Preview allocation only; production must reserve against routes and both hosts.
    for (let octet = 0; octet < 256; octet++) {
      for (let tail = 0; tail < 256; tail += 4) {
        const network = `10.77.${octet}.${tail}/30`;
        if (!usedNetworks.has(network))
          return { name: "exit" + index, network };
      }
    }
    throw new Error("شبکهٔ آزاد در محدودهٔ پیشنهادی پیدا نشد.");
  }
  function validateTunnel(values, tunnels) {
    if (
      !/^[a-z][a-z0-9_-]{0,9}$/.test(values.name) ||
      values.name === "default"
    )
      return "نام باید ۱ تا ۱۰ حرف کوچک انگلیسی، عدد، خط تیره یا زیرخط باشد.";
    if (
      !ipv4(values.remote) ||
      /^(0|127|224|225|226|227|228|229|23\d|24\d|25[0-5])\./.test(values.remote)
    )
      return "آی‌پی خارج معتبر وارد کنید.";
    if (!port(values.publicPort) || !port(values.destinationPort))
      return "پورت باید عددی بین ۱ تا ۶۵۵۳۵ باشد.";
    if (!["tcp", "udp"].includes(values.protocol))
      return "پروتکل پورت معتبر نیست.";
    if (
      tunnels.some(
        (t) => t.serverId === values.serverId && t.name === values.name,
      )
    )
      return "نام تانل روی این ایران استفاده شده است.";
    if (
      tunnels.some(
        (t) => t.serverId === values.serverId && t.remote === values.remote,
      )
    )
      return "این زوج سرور تانل GRE دارد؛ پورت را به همان تانل اضافه کنید.";
    if (
      portConflict(tunnels, values.serverId, values.protocol, values.publicPort)
    )
      return "این پورت برای همین پروتکل روی ایران رزرو شده است.";
    if (
      !Number.isInteger(Number(values.mtu)) ||
      Number(values.mtu) < 576 ||
      Number(values.mtu) > 1476
    )
      return "MTU باید بین ۵۷۶ و ۱۴۷۶ باشد.";
    return "";
  }
  const api = { ipv4, port, portConflict, allocate, validateTunnel };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  root.GreFlowDomain = api;
})(typeof globalThis === "undefined" ? this : globalThis);

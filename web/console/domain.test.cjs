"use strict";
const test = require("node:test");
const assert = require("node:assert/strict");
const D = require("./domain.js");
const tunnel = {
  id: "a",
  serverId: "iran1",
  name: "exit2",
  remote: "203.0.113.2",
  network: "10.77.0.0/30",
  forwards: [{ protocol: "tcp", public: 2021, destination: 2021 }],
};
test("IPv4 and port validation rejects shell material, malformed addresses and bounds", () => {
  for (const s of ["1.2.3.256", "1.2.3", "01.2.3.4", "1.2.3.4;id", "::1", ""])
    assert.equal(D.ipv4(s), false);
  assert.equal(D.ipv4("94.182.151.153"), true);
  for (const s of [0, 65536, "2;id", "-1", "2.1", ""])
    assert.equal(D.port(s), false);
  assert.equal(D.port(2021), true);
});
test("public mapping reservations are per Iran and protocol, not global", () => {
  assert.equal(D.portConflict([tunnel], "iran1", "tcp", 2021), true);
  assert.equal(D.portConflict([tunnel], "iran1", "udp", 2021), false);
  assert.equal(D.portConflict([tunnel], "iran2", "tcp", 2021), false);
  assert.equal(D.portConflict([tunnel], "iran1", "tcp", 2021, "a"), false);
});
test("allocator avoids existing names/subnets within selected host", () => {
  assert.deepEqual(D.allocate([tunnel], "iran1"), {
    name: "exit3",
    network: "10.77.0.4/30",
  });
  assert.deepEqual(D.allocate([tunnel], "iran2"), {
    name: "exit2",
    network: "10.77.0.0/30",
  });
});
test("wizard rejects endpoint-pair duplicates and port collisions", () => {
  const v = {
    serverId: "iran1",
    name: "exit3",
    remote: "203.0.113.3",
    publicPort: "2022",
    destinationPort: "2022",
    protocol: "tcp",
    mtu: "1476",
  };
  assert.equal(D.validateTunnel(v, [tunnel]), "");
  assert.ok(D.validateTunnel({ ...v, publicPort: "2021" }, [tunnel]));
  assert.ok(D.validateTunnel({ ...v, remote: tunnel.remote }, [tunnel]));
  assert.ok(D.validateTunnel({ ...v, name: "default" }, [tunnel]));
  assert.ok(D.validateTunnel({ ...v, mtu: "1500" }, [tunnel]));
  assert.ok(D.validateTunnel({ ...v, protocol: "shell" }, [tunnel]));
});

#!/usr/bin/env python3
"""Populate only the loopback screenshot instance through Balemoh's real API."""
import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import json
import math
import time
from urllib.request import Request, urlopen

ORIGIN = "http://127.0.0.1:8180"
# Deliberately public fixture credential. Never use this in a real deployment.
TOKEN = "synthetic-screenshots-only"
SOURCES = [("container", "atlas"), ("container", "pico"), ("kubernetes", "lab-cluster")]
APPS = [
    ("Grafana", "Metrics and dashboards", "grafana", 2, True),
    ("Jellyfin", "Media library for movies and music", "jellyfin", 0, True),
    ("Immich", "Photo library and backups", "immich", 0, True),
    ("Paperless", "Documents, scanned and searchable", "paperless-ngx", 0, True),
    ("Vaultwarden", "Shared household password vault", "vaultwarden", 0, False),
    ("Home Assistant", "Lights, sensors, and automations", "home-assistant", 1, True),
    ("Forgejo", "Git repositories and projects", "forgejo", 2, True),
    ("Prometheus", "Metric collection and storage", "prometheus", 2, False),
]


def request(method, path, payload=None, authenticated=False):
    headers = {"Content-Type": "application/json"}
    if authenticated:
        headers["Authorization"] = "Bearer " + TOKEN
    data = json.dumps(payload).encode() if payload is not None else None
    with urlopen(Request(ORIGIN + path, data=data, method=method, headers=headers), timeout=15) as response:
        body = response.read()
        return json.loads(body) if body else None


def stamp(value):
    return value.isoformat().replace("+00:00", "Z")


def candidate(app, at):
    name, description, slug, source_index, _ = app
    kind, identity = SOURCES[source_index]
    resource_kind = "Service" if kind == "kubernetes" else "container"
    namespace = "homelab" if kind == "kubernetes" else ""
    parts = [kind, identity, resource_kind, namespace, slug]
    stable = "".join(f"{len(p)}:{p}" for p in parts)
    return {
        "id": hashlib.sha256(stable.encode()).hexdigest(),
        "source": {"kind": kind, "id": identity},
        "resource": {"kind": resource_kind, "namespace": namespace, "name": slug},
        "displayName": name, "description": description,
        "metadata": {"fixture": "synthetic"}, "images": [],
        "endpoints": [{"name": "https", "url": f"https://{slug}.example.test",
                       "port": 443, "protocol": "HTTPS", "provenance": "synthetic"}],
        "observedAt": stamp(at),
    }


def observation(source_index, at, node=""):
    minute = at.timestamp() / 60
    phase = minute / 7 + source_index * 2
    cpu = 22 + source_index * 5 + 9 * math.sin(phase) + 4 * math.sin(phase * 2.7)
    memory = 49 + source_index * 4 + 7 * math.sin(phase / 4)
    total = (32 if source_index != 1 else 8) * 1024**3
    workloads = [{"name": slug if source_index != 2 else slug + "-6bc9d7f4-abcde",
                  "namespace": "homelab" if source_index == 2 else "",
                  "state": "Running" if source_index == 2 else "running", "restarts": 0}
                 for _, _, slug, index, _ in APPS if index == source_index]
    return {"node": node, "architecture": "arm64" if source_index == 1 else "amd64",
            "ready": "True" if node else "", "inventoryAvailable": True,
            "cpuPercent": round(cpu, 1), "memoryUsedBytes": int(total * memory / 100),
            "memoryTotalBytes": total, "workloads": workloads}


def telemetry(at):
    for index, (kind, identity) in enumerate(SOURCES):
        hosts = [observation(index, at)]
        if kind == "kubernetes":
            hosts[0].pop("architecture")
            hosts += [observation(index, at, node="worker-01")]
        request("POST", "/api/v1/federation/telemetry",
                {"source": {"kind": kind, "id": identity}, "observedAt": stamp(at), "hosts": hosts}, True)


def seed():
    now = datetime.now(timezone.utc)
    for index, (kind, identity) in enumerate(SOURCES):
        candidates = [candidate(app, now) for app in APPS if app[3] == index]
        request("POST", "/api/v1/federation/snapshots",
                {"source": {"kind": kind, "id": identity}, "observedAt": stamp(now),
                 "candidates": candidates}, True)
    services = request("GET", "/api/v1/staging/services")["services"]
    expected = {app[0]: app for app in APPS}
    assert len(services) == len(expected), "Use a fresh screenshot database"
    for service in services:
        name, description, slug, _, pinned = expected[service["displayName"]]
        path = "/api/v1/staging/services/" + service["id"]
        request("PUT", path, {"displayName": name, "description": description,
                              "address": "", "icon": "selfhst:" + slug})
        request("POST" if pinned else "DELETE", path + "/pin")
    for offset in range(120, -1, -1):
        telemetry(now - timedelta(minutes=offset))
    hosts = request("GET", "/api/v1/hosts")["hosts"]
    for host in hosts:
        if not host["node"]:
            request("PUT", "/api/v1/dashboard/widgets/host/" + host["id"],
                    {"enabled": host["source"]["kind"] == "container"})
    assert len(request("GET", "/api/v1/homepage/services")["services"]) == 6
    print("Seeded 8 synthetic services, 2 host widgets, and 121 minutes of telemetry.", flush=True)
    print("UI: http://127.0.0.1:8181/", flush=True)
    print("Host: http://127.0.0.1:8181/hosts/" + next(h["id"] for h in hosts if h["name"] == "atlas"), flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--keep-fresh", action="store_true", help="Continue emitting synthetic reports each minute")
    args = parser.parse_args()
    seed()
    while args.keep_fresh:
        time.sleep(60)
        telemetry(datetime.now(timezone.utc))

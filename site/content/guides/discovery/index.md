---
title: Automatic service discovery
description: Discover Kubernetes and container workloads without automatically publishing them to your homepage.
language: en
---

# Automatic service discovery

Balemoh finds services in Kubernetes and in Docker or Podman containers, so you
can review new apps without entering every service by hand. Discovery creates
candidates in **Staging**. You choose which ones appear on your homepage.

## Choose what appears on your homepage

1. Enable a discovery source using the setup below, then open **Staging**.
2. Review the **Live** entries. Check each service's address before opening it.
3. Open a service's editor to change its display name, description, address, or
   icon. These choices survive later discovery refreshes.
4. Select its pin checkbox. Return to the homepage to see its service card and
   find it with [service search](../search/index.md).

![The real Balemoh Staging UI with eight synthetic services and homepage pin checkboxes.](../../media/staging-synthetic.jpg)

This screenshot uses synthetic service names and addresses in a local instance.

Uncheck the pin to remove a card from the homepage. Use **Hide** to move an entry
out of Live while retaining its saved settings. **Show** restores it to Live or
Missing according to its current presence. These actions change Balemoh's catalog;
they do not stop containers or change Kubernetes resources.

For an app outside your discovery sources, choose **Add service** in Staging.
Enter a name and an absolute HTTP or HTTPS address, then save and pin it.
Manual services stay live across discovery refreshes.

## Docker and Podman

Run the API on a host with a Docker-compatible socket:

```sh
BALEMOH_CONTAINER_ENABLED=true \
BALEMOH_CONTAINER_SOURCE_ID=docker-local \
BALEMOH_CONTAINER_HOST=unix:///var/run/docker.sock \
go run ./cmd/balemoh
```

For rootless Podman, use the user Podman socket. The adapter reads running
containers and Compose labels. It records image and published-port evidence.
A published TCP port is not proof of an HTTP URL or hostname.

The socket remains a privileged host capability, even though discovery performs
list requests. Isolate its access and use a stable source identity.

## Kubernetes

Enable the Kubernetes source on an API process running in-cluster. Give its
ServiceAccount read-only access to Pods, Services, Nodes, Ingresses, and
HTTPRoutes. The UI needs no Kubernetes token.

The discoverer resolves HTTPRoute and Ingress backends to Services, then matches
Pods through Service label selectors. Without a resolved route it falls back to
Services. ExternalName, selectorless, and externally addressed Services can be
staged without a Pod. Namespace scope is configurable.

Balemoh retains route, Gateway, listener, path, and redirect evidence.
It groups relationships only when that evidence is unambiguous. A shared
namespace, label, or hostname alone does not prove two resources are one app.

## Refresh and review

With background synchronization enabled, Balemoh scans at startup and on the
configured interval. Use **Sync discovery** in Staging to request a scan sooner.
Remote agents publish their own snapshots; for several machines, follow
[Federated agents](../federation/index.md).

The **Missing** tab holds services absent from a successful, complete source
snapshot. A failed scan leaves that source's previous state unchanged. Missing
services leave the homepage; when rediscovered, their saved edits and pin choices
return with them. **Hidden** entries retain their visibility choice across scans.

Only missing services can be permanently deleted. Deletion removes their saved
settings and pins. If discovered again later, they start with discovery defaults.
Hide a live service when you want to keep it out of the way.

If Staging is empty, check that a source is enabled and its socket or Kubernetes
permissions are available. If a service has no usable URL, review the address
evidence or set an explicit address in its editor. Discovery does not test whether
an address serves a healthy application.

The [generated configuration](../../reference/index.md) lists exact variable names
and defaults. The [API reference](https://balemoh.araihu.com/openapi/catalogs/balemoh/documents/openapi/)
documents discovery, staging, and pin operations.

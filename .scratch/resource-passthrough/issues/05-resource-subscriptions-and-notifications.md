# 05 — Resource subscriptions and change notifications

**Type:** task

**What to build:** A session can subscribe to a resource and receive updates for it, and clients that track resource lists are told when the catalog changes. Upstream notifications are translated into bridge-namespaced notifications and delivered only to interested sessions.

**Blocked by:** 01.

**Status:** ready-for-agent

- [ ] `resources/subscribe` / `resources/unsubscribe` are tracked per session; at most one upstream subscription exists per (server, raw URI) per session.
- [ ] `notifications/resources/updated` is translated to the bridge URI and delivered only to sessions subscribed to that URI; a session that subscribed to a different resource receives nothing.
- [ ] `notifications/resources/list_changed` triggers a resource catalog refresh and is delivered to sessions that requested list-change notifications, once per refresh.
- [ ] Unsubscribe and session teardown release upstream subscription state; an upstream disconnect clears the mapping without emitting phantom notifications.
- [ ] Subscription counts are bounded per session and per server; exceeding the bound is rejected explicitly.
- [ ] Tests cover two sessions with distinct subscriptions, unsubscribe, session teardown, and upstream disconnect.
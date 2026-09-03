# wacli Feature Verification Map

This directory maps the core verifiable features of `wacli`. Each feature file details how a user experiences the feature, how an agent can programmatically drive it with the harness, and what observable state proves it works.

## Indexed Features

| Feature | Surface / Command | Description |
| :--- | :--- | :--- |
| [`groups-participants.md`](groups-participants.md) | `wacli groups participants list` | Inspects members and roles of a WhatsApp group from local DB |
| [`chats-mark-read-ipc.md`](chats-mark-read-ipc.md) | `wacli chats mark-read` | Marks chat as read via IPC follow socket while daemon runs |
| [`messages-search.md`](messages-search.md) | `wacli messages search` | Full-text FTS5 search across local messages |
| [`doctor-diagnostics.md`](doctor-diagnostics.md) | `wacli doctor` | Store health, lock state, connection, and schema check |

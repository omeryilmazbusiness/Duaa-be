# DUA-OS-BE

Duaa Executive OS backend. Go + PostgreSQL.

Guest Premium’s **GM / müdür / çalışan** kernel, cafe-first. Hotel and supply catalogs are registered extension points; they are not the first delivery.

## Roles

| Guest-pro | `scope` | Cafe |
|-----------|---------|------|
| GM | `general_manager` | Duaa |
| Department manager | `department_manager` | Kitchen / bar / service müdürü |
| Employee number portal | `staff_personnel` | Çalışan |
| Reception analog | `floor_ops` | SERVICE |

Cafe GM tabs: `today`, `ops`, `dashboard`, `feedback`.

## Self-test

```bash
cd /Users/omeryilmaz/Development/DUA-OS-BE
make self-test
```

Memory demo (no Postgres): `make run`  
GM: `duaa@example.com` / `ChangeMe123!` — cafe slug `duaa-cafe`.

Frontend (Next.js, Guest Premium shell): [DUA-OS-FE](../DUA-OS-FE). `cd ../DUA-OS-FE && npm run dev` on port 3000; it rewrites `/v1` to this API.

# New features

New features should be discussed. Open an issue before trying anything major.
New features are not free to maintain and put a burden on the maintainers of the
project, notably when it comes to fixing bugs and when they interfere with future
evolutions.

# User friendliness

Network people are usually less savvy when it comes to complex systems. There
are three pillars that *Akvorado* follows to make it easier for its target
users:

- `docker compose` to get started quickly for most setups
- easy upgrades through automatic migrations (database and configuration)
- documentation including configuration, exploitation, and troubleshooting

# Style guide

Go formatter takes care of most issues. For the remaining points:

- comments are sentences and should be capitalized
- on the other hand, log messages are not and should *not* be capitalized
- metrics should be named using [Prometheus conventions][]

[prometheus conventions]: https://prometheus.io/docs/practices/naming/

Git commits are prefixed with the component and sub-component of the feature:
`orchestrator/clickhouse: add some feature`. Meta-component are also possible,
like `docs`, `build`, or `docker`.

# Testing

We do not aim for 100% code coverage, however most code should be covered by
tests. This is a big task, but it pays when adding new features or refactoring.
The test suite should run quick enough to not become a burden.

Use `make test-go` to run Go tests. You can restrict it to a specific package
with `make test-go PKG=akvorado/orchestrator/clickhouse`. Using just `go test`
would work, but `make test-go` also runs linting and formatting automatically.

If possible, tests should not rely on external components, but when it becomes
hard to do so, it is possible to spawn services through Docker. Locally, one
can spawn them through `docker compose -f docker/docker-compose-dev.yml`:

- `... up clickhouse` to spawn a single ClickHouse
- `... up clickhouse-\*` to spawn a ClickHouse cluster
- `... up kafka` to spawn a Kafka broker

# Hacking

For manual tests, you can use `make docker-dev` to build a Docker container,
then use `docker compose --profile demo up` to run Docker Compose. Each time you
modify the code, repeat these two steps. Beware not to destroy the volume for
GeoIP at each attempt as there is a per-day limit on the number of times one IP
can fetch the GeoIP database.

When using this method, it can take a little bit of time until the console is
declared healthy. In the meantime, you get a 404 error. If you want to avoid
that, you can the console locally instead:

```console
$ docker compose stop akvorado-console
$ make && AKVORADO_CFG_CONSOLE_CLICKHOUSE_SERVERS=$(docker container inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' akvorado-clickhouse-1):9000 AKVORADO_CFG_CONSOLE_SERVELIVEFS=true ./bin/akvorado console /dev/null
```

If you need to work on the frontend part, you can spawn the Docker compose
setup, then in `console/frontend`, use `npm run dev` and point your browser to
`http://localhost:5173` instead of `http://localhost:8080`. Any change of
frontend-related files should be applied immediately.

# Licensing

The code is licensed under AGPL-3.0-only. When creating new files, be sure to
add the appropriate SPDX header, like for existing files. Feel free to assign
the copyright to yourself or your organization: we do not do copyright
assignment as GitHub terms and conditions already [include][] this:

> Whenever you add Content to a repository containing notice of a license, you
> license that Content under the same terms, and you agree that you have the
> right to license that Content under those terms.

[include]: https://docs.github.com/en/site-policy/github-terms/github-terms-of-service#6-contributions-under-repository-license

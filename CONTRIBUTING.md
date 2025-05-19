# New features

New features should be discussed. Open an issue before trying anything major.
New features are not free to maintain and put a burden on the maintainers of the
project, notably when it comes to fix bugs and when they interfer in future
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

- comments are sentence and should be capitalized
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

If possible, tests should not rely on external components, but when it becomes
hard to do so, it is possible to spawns services through Docker. Locally, one
can spawns them through `docker compose -f docker/docker-compose-dev.yml`.
GitHub actions are using services to spawn them.

For manual tests, you can use `make docker-dev` to build a Docker container of
Akvorado, then use `docker compose up` to run Docker compose. Beware to not
destroy the volume for GeoIP at each tentative as there is a per-day limit on
the number of times one IP can fetch the GeoIP database.

If you need to work on the frontend part, you can spawn the Docker compsoe
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

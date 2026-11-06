COMMENT =	flow collector, enricher and visualizer
V =		2026.10.1.20261104
DISTNAME =	akvorado-${V}
CATEGORIES =	net

GH_ACCOUNT =	cleberalves40
GH_PROJECT =	akvorado
GH_COMMIT =	472d8382d88e61f5181c13880b02e2808480075c

HOMEPAGE =	https://github.com/cleberalves40/akvorado
PERMIT_PACKAGE = Yes

MODULES =	lang/go
MODGO_GO111MODULE = off
# WIP: Go module staging and generated embedded assets remain unresolved.
UNLINKED = wip

MAKE_ENV +=	CGO_ENABLED=0 GOTOOLCHAIN=local XDG_CONFIG_HOME=${WRKDIR}

.include <bsd.port.mk>

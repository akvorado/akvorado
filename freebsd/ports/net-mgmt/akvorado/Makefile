PORTNAME= akvorado
DISTVERSION= 2026.10.1.20261104
CATEGORIES= net-mgmt

MAINTAINER= ports@FreeBSD.org
COMMENT= Flow collector, enricher and visualizer
WWW= https://github.com/cleberalves40/akvorado

LICENSE= AGPLv3
LICENSE_FILE= ${WRKSRC}/LICENSE.txt

USES= go:1.27+,modules,no_targets
GO_ENV+= GOTELEMETRY=off XDG_CONFIG_HOME=/tmp
USE_GITHUB= yes
GH_ACCOUNT= cleberalves40
GH_TAGNAME= 472d8382d88e61f5181c13880b02e2808480075c

GO_MODULE= akvorado@${GH_TAGNAME}
GO_MOD_DIST= github
GO_LDFLAGS+= -w -X akvorado/common/helpers.AkvoradoVersion=${PORTVERSION}

USE_RC_SUBR= akvorado_orchestrator akvorado_inlet akvorado_outlet akvorado_console
SUB_FILES= pkg-message
SUB_LIST= AKVORADO_USER=${USERS} PREFIX=${PREFIX} ETCDIR=${ETCDIR}

USERS= akvorado
GROUPS= akvorado
UID_FILES= ${FILESDIR}/UIDs
GID_FILES= ${FILESDIR}/GIDs

OPTIONS_DEFINE= DOCS
PORTDOCS= README.md

post-extract:
	${TAR} -xzf ${FILESDIR}/generated-assets.tar.gz -C ${WRKSRC}

do-build:
	${MKDIR} ${WRKSRC}/bin
	(cd ${WRKSRC} && ${SETENVI} ${WRK_ENV} ${MAKE_ENV} ${GO_ENV} \
		CGO_ENABLED=0 GOMAXPROCS=${MAKE_JOBS_NUMBER} GOPROXY=off \
		${GO_CMD} build ${GO_BUILDFLAGS} \
		-tags=release,grpcnotrace,validator_novalidatefn \
		-o bin/akvorado ./cmd/akvorado)
	(cd ${WRKSRC} && ${SETENVI} ${WRK_ENV} ${MAKE_ENV} ${GO_ENV} \
		CGO_ENABLED=0 GOMAXPROCS=${MAKE_JOBS_NUMBER} GOPROXY=off \
		${GO_CMD} build ${GO_BUILDFLAGS} \
		-tags=release,grpcnotrace,validator_novalidatefn,akvorado_inlet \
		-o bin/akvorado-inlet ./cmd/akvorado)

do-install:
	${INSTALL_PROGRAM} ${WRKSRC}/bin/akvorado ${STAGEDIR}${PREFIX}/bin
	${INSTALL_PROGRAM} ${WRKSRC}/bin/akvorado-inlet ${STAGEDIR}${PREFIX}/bin
	${MKDIR} ${STAGEDIR}${ETCDIR}
.for config in akvorado.yaml console.yaml inlet.yaml outlet.yaml
	${INSTALL_DATA} ${WRKSRC}/config/${config} ${STAGEDIR}${ETCDIR}/${config}.sample
.endfor

do-install-DOCS-on:
	${MKDIR} ${STAGEDIR}${DOCSDIR}
	(cd ${WRKSRC} && ${COPYTREE_SHARE} "${PORTDOCS}" ${STAGEDIR}${DOCSDIR})

.include <bsd.port.mk>
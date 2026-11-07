PORTNAME=	akvorado
DISTVERSION=	2026.10.1.20261104
CATEGORIES=	net-mgmt
MASTER_SITES=	https://github.com/cleberalves40/akvorado/releases/download/bsd-distfiles-${DISTVERSION}/:assets
DISTFILES=	${PORTNAME}-${DISTVERSION}-generated-assets.tar.gz:assets

MAINTAINER=	cleber@bsd.com.br
COMMENT=	Flow collector, enricher and visualizer
WWW=		https://github.com/akvorado/akvorado

LICENSE=	AGPLv3
LICENSE_FILE=	${WRKSRC}/LICENSE.txt

USES=		go:1.27+,modules,no_targets
USE_GITHUB=	yes
GH_ACCOUNT=	akvorado
GH_TAGNAME=	472d8382d88e61f5181c13880b02e2808480075c
USE_RC_SUBR=	akvorado_console akvorado_inlet akvorado_orchestrator \
		akvorado_outlet

GO_MODULE=	akvorado@${GH_TAGNAME}
GO_MOD_DIST=	github
CGO_ENABLED=	0
GO_ENV+=	GOTELEMETRY=off XDG_CONFIG_HOME=${WRKDIR}
GO_LDFLAGS+=	-w -X akvorado/common/helpers.AkvoradoVersion=${PORTVERSION}
_AKVORADO_TAGS=	release,grpcnotrace,validator_novalidatefn

SUB_FILES=	pkg-message
SUB_LIST=	AKVORADO_USER=${USERS}

USERS=		akvorado
GROUPS=		akvorado

PORTDOCS=	README.md

OPTIONS_DEFINE=	DOCS

_AKVORADO_CONFIGS=	akvorado.yaml console.yaml inlet.yaml outlet.yaml

# The upstream sample configuration targets the Docker Compose setup. Adapt
# it to a single host: local Kafka/ClickHouse, one HTTP port per service,
# memory cache instead of Redis, one UDP worker per port (SO_REUSEPORT does
# not load-balance on FreeBSD) and the GeoIP location used by ports.
post-patch:
	@${REINPLACE_CMD} \
		-e 's|- kafka:9092|- 127.0.0.1:9092|' \
		-e 's|- clickhouse:9000|- 127.0.0.1:9000|' \
		-e 's|http://akvorado-orchestrator:8080|http://127.0.0.1:8080|' \
		${WRKSRC}/config/akvorado.yaml
	@${REINPLACE_CMD} -e 's|workers: 4|workers: 1|' \
		${WRKSRC}/config/inlet.yaml
	@${REINPLACE_CMD} -e 's|/usr/share/GeoIP|${LOCALBASE}/share/GeoIP|' \
		${WRKSRC}/config/outlet.yaml
	@${REINPLACE_CMD} \
		-e 's|^  cache:$$|  listen: :8083|' \
		-e '/^    type: redis$$/d' \
		-e '/^    server: redis:6379$$/d' \
		${WRKSRC}/config/console.yaml
	@${PRINTF} 'http:\n  listen: :8081\n' >> ${WRKSRC}/config/inlet.yaml
	@${PRINTF} 'http:\n  listen: :8082\n' >> ${WRKSRC}/config/outlet.yaml

do-build:
	(cd ${GO_WRKSRC} && ${SETENVI} ${WRK_ENV} ${MAKE_ENV} ${GO_ENV} \
		GOMAXPROCS=${MAKE_JOBS_NUMBER} GOPROXY=off \
		${GO_CMD} build ${GO_BUILDFLAGS} \
		-tags=${_AKVORADO_TAGS} -o ${GO_WRKDIR_BIN}/akvorado \
		./cmd/akvorado)
	(cd ${GO_WRKSRC} && ${SETENVI} ${WRK_ENV} ${MAKE_ENV} ${GO_ENV} \
		GOMAXPROCS=${MAKE_JOBS_NUMBER} GOPROXY=off \
		${GO_CMD} build ${GO_BUILDFLAGS} \
		-tags=${_AKVORADO_TAGS},akvorado_inlet \
		-o ${GO_WRKDIR_BIN}/akvorado-inlet ./cmd/akvorado)

do-install:
	${INSTALL_PROGRAM} ${GO_WRKDIR_BIN}/akvorado \
		${GO_WRKDIR_BIN}/akvorado-inlet ${STAGEDIR}${PREFIX}/bin
	@${MKDIR} ${STAGEDIR}${ETCDIR}
.for config in ${_AKVORADO_CONFIGS}
	${INSTALL_DATA} ${WRKSRC}/config/${config} \
		${STAGEDIR}${ETCDIR}/${config}.sample
.endfor

do-install-DOCS-on:
	@${MKDIR} ${STAGEDIR}${DOCSDIR}
	${INSTALL_DATA} ${WRKSRC}/README.md ${STAGEDIR}${DOCSDIR}

.include <bsd.port.mk>

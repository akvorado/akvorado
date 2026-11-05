# $NetBSD$

DISTNAME=       akvorado-${GITHUB_TAG}
PKGNAME=        akvorado-2026.10.1.20261104
CATEGORIES=     net
MASTER_SITES=   ${MASTER_SITE_GITHUB:=cleberalves40/}
GITHUB_PROJECT= akvorado
GITHUB_TAG=     472d8382d88e61f5181c13880b02e2808480075c

MAINTAINER=     pkgsrc-users@NetBSD.org
HOMEPAGE=       https://github.com/cleberalves40/akvorado
COMMENT=        Flow collector, enricher and visualizer
LICENSE=        gnu-agpl-v3

GO_VERSION_REQD= 127
PKG_SYSCONFDIR= ${PREFIX}/etc/akvorado
EGDIR=          ${PREFIX}/share/examples/akvorado

CONF_FILES+=    ${EGDIR}/akvorado.yaml ${PKG_SYSCONFDIR}/akvorado.yaml
CONF_FILES+=    ${EGDIR}/console.yaml ${PKG_SYSCONFDIR}/console.yaml
CONF_FILES+=    ${EGDIR}/inlet.yaml ${PKG_SYSCONFDIR}/inlet.yaml
CONF_FILES+=    ${EGDIR}/outlet.yaml ${PKG_SYSCONFDIR}/outlet.yaml

INSTALLATION_DIRS+=     bin share/doc/akvorado share/examples/akvorado
MAKE_ENV+=              GOTELEMETRY=off XDG_CONFIG_HOME=${WRKDIR}

.include "go-modules.mk"
.include "../../lang/go/go-module.mk"

post-extract:
	${RUN} ${TAR} -xzf ${FILESDIR}/generated-assets.tar.gz -C ${WRKSRC}

do-build:
	${RUN} ${MKDIR} ${WRKDIR}/.gopath/bin
	${RUN} cd ${WRKSRC} && ${PKGSRC_SETENV} ${MAKE_ENV} CGO_ENABLED=0 \
		${GO} build -trimpath -tags=release,grpcnotrace,validator_novalidatefn \
		-ldflags "-w -X akvorado/common/helpers.AkvoradoVersion=${PKGVERSION_NOREV}" \
		-o ${WRKDIR}/.gopath/bin/akvorado ./cmd/akvorado
	${RUN} cd ${WRKSRC} && ${PKGSRC_SETENV} ${MAKE_ENV} CGO_ENABLED=0 \
		${GO} build -trimpath -tags=release,grpcnotrace,validator_novalidatefn,akvorado_inlet \
		-ldflags "-w -X akvorado/common/helpers.AkvoradoVersion=${PKGVERSION_NOREV}" \
		-o ${WRKDIR}/.gopath/bin/akvorado-inlet ./cmd/akvorado

post-install:
	${RUN} ${INSTALL_DATA} ${WRKSRC}/README.md ${DESTDIR}${PREFIX}/share/doc/akvorado/README.md
.for config in akvorado.yaml console.yaml inlet.yaml outlet.yaml
	${RUN} ${INSTALL_DATA} ${WRKSRC}/config/${config} ${DESTDIR}${EGDIR}/${config}
.endfor

.include "../../mk/bsd.pkg.mk"
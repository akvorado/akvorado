#!@RCD_SCRIPTS_SHELL@
#
# $NetBSD$
#
# PROVIDE: akvorado_orchestrator
# REQUIRE: DAEMON
# KEYWORD: shutdown
#
# To start the Akvorado orchestrator at boot, add to /etc/rc.conf:
#
#	akvorado_orchestrator=YES
#
# Optional settings (defaults shown):
#
#	akvorado_user="@AKVORADO_USER@"
#	akvorado_orchestrator_config="@PKG_SYSCONFDIR@/akvorado.yaml"

if [ -f /etc/rc.subr ]; then
	. /etc/rc.subr
fi

name="akvorado_orchestrator"
rcvar=$name
command="@PREFIX@/bin/akvorado"
pidfile="@VARBASE@/run/akvorado/orchestrator.pid"
logfile="@VARBASE@/log/akvorado/orchestrator.log"
start_cmd="akvorado_start"

# Akvorado runs in the foreground: start it in the background as the
# unprivileged user and record its PID so that stop/status target this
# service only (orchestrator, outlet and console share the same binary).
akvorado_start()
{
	echo "Starting ${name}."
	/usr/bin/su -m ${akvorado_user} -c \
	    "${command} orchestrator ${akvorado_orchestrator_config} >>${logfile} 2>&1 & echo \$! >${pidfile}"
}

load_rc_config $name
: ${akvorado_user:="@AKVORADO_USER@"}
: ${akvorado_orchestrator_config:="@PKG_SYSCONFDIR@/akvorado.yaml"}
required_files="${akvorado_orchestrator_config}"
run_rc_command "$1"

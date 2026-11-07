#!@RCD_SCRIPTS_SHELL@
#
# $NetBSD$
#
# PROVIDE: akvorado_console
# REQUIRE: DAEMON akvorado_orchestrator
# KEYWORD: shutdown
#
# To start the Akvorado console at boot, add to /etc/rc.conf:
#
#	akvorado_console=YES
#
# Optional settings (defaults shown):
#
#	akvorado_user="@AKVORADO_USER@"
#	akvorado_console_url="http://127.0.0.1:8080"

if [ -f /etc/rc.subr ]; then
	. /etc/rc.subr
fi

name="akvorado_console"
rcvar=$name
command="@PREFIX@/bin/akvorado"
pidfile="@VARBASE@/run/akvorado/console.pid"
logfile="@VARBASE@/log/akvorado/console.log"
start_cmd="akvorado_start"

# Akvorado runs in the foreground: start it in the background as the
# unprivileged user and record its PID so that stop/status target this
# service only (orchestrator, outlet and console share the same binary).
akvorado_start()
{
	echo "Starting ${name}."
	/usr/bin/su -m ${akvorado_user} -c \
	    "${command} console ${akvorado_console_url} >>${logfile} 2>&1 & echo \$! >${pidfile}"
}

load_rc_config $name
: ${akvorado_user:="@AKVORADO_USER@"}
: ${akvorado_console_url:="http://127.0.0.1:8080"}
run_rc_command "$1"

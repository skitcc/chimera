#!/bin/sh
# Three systemd slices with disjoint AllowedCPUs. Containers are also pinned
# with Docker --cpuset-cpus to the same sets: minikube on the application
# slice, k6 on the load slice, Prometheus and Grafana on the observe slice.
set -eu

state_dir=${PERF_STATE:-/tmp/chimera-perf-isolate-$(id -u)}
unit_dir=/run/systemd/system

die() {
	printf '%s\n' "$*" >&2
	exit 1
}

need_root() {
	[ "$(id -u)" -eq 0 ] || die "isolate.sh needs root (sudo sh deploy/perf/isolate.sh)"
	command -v systemctl >/dev/null 2>&1 || die "systemd is required"
	command -v docker >/dev/null 2>&1 || die "docker is required"
}

ncpus() {
	nproc
}

layout() {
	n=$(ncpus)
	[ "$n" -ge 4 ] || die "need at least 4 CPUs, have $n"
	if [ "$n" -ge 8 ]; then
		app_cpus=0-3
		load_cpus=4-5
		observe_cpus=6-7
	else
		last_app=$((n - 3))
		app_cpus=0-$last_app
		load_cpus=$((n - 2))
		observe_cpus=$((n - 1))
	fi
}

cpus_of() {
	slot=$1
	# shellcheck disable=SC1091
	. "$state_dir/layout"
	case $slot in
	app) printf '%s\n' "$app_cpus" ;;
	load) printf '%s\n' "$load_cpus" ;;
	observe) printf '%s\n' "$observe_cpus" ;;
	*) die "unknown slot $slot" ;;
	esac
}

write_slice() {
	slot=$1
	cpus=$2
	file=$unit_dir/chimera-perf-$slot.slice
	mkdir -p "$unit_dir"
	cat > "$file" << EOF
[Unit]
Description=Chimera perf $slot CPUs

[Slice]
AllowedCPUs=$cpus
EOF
}

apply_layout() {
	layout
	docker inspect minikube >/dev/null 2>&1 || die "minikube container is not running"
	mkdir -p "$state_dir"
	printf 'app_cpus=%s\nload_cpus=%s\nobserve_cpus=%s\nncpus=%s\n' \
		"$app_cpus" "$load_cpus" "$observe_cpus" "$(ncpus)" > "$state_dir/layout"
	orig=$(docker inspect -f '{{.HostConfig.CpusetCpus}}' minikube)
	printf '%s\n' "$orig" > "$state_dir/minikube.cpuset"

	if [ "$(id -u)" -ne 0 ]; then
		command -v taskset >/dev/null 2>&1 || die "taskset is required without root"
		docker update --cpuset-cpus="$app_cpus" minikube >/dev/null
		printf 'isolation=docker-cpuset-and-taskset\n' > "$state_dir/scope.txt"
		check_container minikube app
		return
	fi
	write_slice app "$app_cpus"
	write_slice load "$load_cpus"
	write_slice observe "$observe_cpus"
	systemctl daemon-reload
	systemctl start chimera-perf-app.slice chimera-perf-load.slice chimera-perf-observe.slice

	docker update --cpuset-cpus="$app_cpus" minikube >/dev/null
	id=$(docker inspect -f '{{.Id}}' minikube)
	if systemctl set-property "docker-${id}.scope" AllowedCPUs="$app_cpus" >/dev/null 2>"$state_dir/scope.err"; then
		printf 'minikube_scope=set\n' > "$state_dir/scope.txt"
	else
		printf 'minikube_scope=docker-cpuset-only\n' > "$state_dir/scope.txt"
	fi
	check_container minikube app
}

check_container() {
	name=$1
	slot=$2
	want=$(cpus_of "$slot")
	got=$(docker inspect -f '{{.HostConfig.CpusetCpus}}' "$name")
	[ "$got" = "$want" ] || die "container $name cpuset is '$got', want '$want'"
	printf '%s cpuset %s role %s\n' "$name" "$got" "$slot"
}

run_unit() {
	slot=$1
	unit=$2
	shift 2
	[ "$1" = "--" ] || die "usage: isolate.sh run SLOT UNIT -- command"
	shift
	cpus=$(cpus_of "$slot")
	if [ "$(id -u)" -ne 0 ]; then
		taskset -c "$cpus" "$@" </dev/null >"$state_dir/$unit.log" 2>&1 &
		printf '%s\n' "$!" > "$state_dir/$unit.pid"
		return
	fi
	need_root
	systemctl stop "chimera-perf-$unit.service" >/dev/null 2>&1 || true
	systemctl reset-failed "chimera-perf-$unit.service" >/dev/null 2>&1 || true
	# Transient units do not inherit this shell's environment. kubectl proxy
	# needs the same minikube profile the scenario uses.
	set -- \
		--unit="chimera-perf-$unit" \
		--slice="chimera-perf-$slot.slice" \
		--property=AllowedCPUs="$cpus" \
		--property=StandardInput=null \
		--collect \
		--setenv="PATH=${PATH:-/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin}" \
		"$@"
	if [ -n "${MINIKUBE_HOME:-}" ]; then
		set -- --setenv="MINIKUBE_HOME=$MINIKUBE_HOME" "$@"
	fi
	if [ -n "${KUBECONFIG:-}" ]; then
		set -- --setenv="KUBECONFIG=$KUBECONFIG" "$@"
	fi
	systemd-run "$@"
}

release() {
	if [ "$(id -u)" -ne 0 ]; then
		# The Python runner owns its process groups. Never kill stale PID files:
		# the OS may have reused those numbers since an earlier invocation.
		if [ -f "$state_dir/minikube.cpuset" ]; then
			orig=$(cat "$state_dir/minikube.cpuset")
			[ -n "$orig" ] || orig=0-$(($(ncpus) - 1))
			docker update --cpuset-cpus="$orig" minikube >/dev/null
		fi
		return
	fi
	need_root
	systemctl stop chimera-perf-proxy.service chimera-perf-iostat.service chimera-perf-uptime.service >/dev/null 2>&1 || true
	if [ -f "$state_dir/minikube.cpuset" ] && docker inspect minikube >/dev/null 2>&1; then
		orig=$(cat "$state_dir/minikube.cpuset")
		if [ -z "$orig" ]; then
			orig=0-$(($(ncpus) - 1))
		fi
		docker update --cpuset-cpus="$orig" minikube >/dev/null 2>&1 || true
		id=$(docker inspect -f '{{.Id}}' minikube)
		systemctl revert "docker-${id}.scope" >/dev/null 2>&1 || true
	fi
	systemctl stop chimera-perf-app.slice chimera-perf-load.slice chimera-perf-observe.slice >/dev/null 2>&1 || true
	rm -f "$unit_dir/chimera-perf-app.slice" "$unit_dir/chimera-perf-load.slice" "$unit_dir/chimera-perf-observe.slice"
	systemctl daemon-reload >/dev/null 2>&1 || true
	printf 'isolation released\n'
}

usage() {
	printf '%s\n' "usage: isolate.sh apply|cpus SLOT|check NAME SLOT|run SLOT UNIT -- command|release|layout"
	exit 2
}

cmd=${1:-}
case $cmd in
apply) apply_layout ;;
cpus) cpus_of "$2" ;;
check) check_container "$2" "$3" ;;
run)
	shift
	run_unit "$@"
	;;
release) release ;;
layout)
	layout
	printf 'app=%s load=%s observe=%s\n' "$app_cpus" "$load_cpus" "$observe_cpus"
	;;
*) usage ;;
esac

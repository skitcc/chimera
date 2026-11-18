#!/bin/sh
# Moves the records of identity.user.v1.dlq and catalog.track.v1.dlq back to their
# topics once the cause is fixed. Runs the Social binary in replay mode inside a
# running pod, with that pod's broker settings. Consumers dedupe by event id, so a
# replayed record that some group already handled is skipped there.
set -eu

kubectl=${KUBECTL:-kubectl}
namespace=${K8S_NAMESPACE:-chimera}

"$kubectl" -n "$namespace" exec deploy/social -- /service -replay-dlq

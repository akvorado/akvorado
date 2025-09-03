# Test last flow widget
GET http://127.0.0.1:8080/api/v0/console/widget/flow-last
HTTP 200
[Asserts]
jsonpath "$.InIfName" matches /^Gi0\/.*/
jsonpath "$.OutIfName" matches /^Gi0\/.*/
jsonpath "$.ExporterRole" == "edge"
jsonpath "$.SamplingRate" == 100000

# Test metrics
GET http://127.0.0.1:8080/api/v0/console/metrics
HTTP 200
[Asserts]
body matches /^# HELP akvorado_cmd_info Akvorado build information/
GET http://127.0.0.1:8080/api/v0/inlet/metrics
HTTP 200
[Asserts]
body matches /^# HELP akvorado_cmd_info Akvorado build information/
GET http://127.0.0.1:8080/api/v0/outlet/metrics
HTTP 200
[Asserts]
body matches /^# HELP akvorado_cmd_info Akvorado build information/
GET http://127.0.0.1:8080/api/v0/orchestrator/metrics
HTTP 200
[Asserts]
body matches /^# HELP akvorado_cmd_info Akvorado build information/

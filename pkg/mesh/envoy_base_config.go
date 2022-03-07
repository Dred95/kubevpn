package mesh

var s = `
node:
  cluster: %s
  id: %s
admin:
  access_log_path: /dev/null
  address:
    socket_address:
      address: 0.0.0.0
      port_value: 9003
dynamic_resources:
  ads_config:
    api_type: GRPC
    transport_api_version: V3
    grpc_services:
    - envoy_grpc:
        cluster_name: xds_cluster
    set_node_on_first_message_only: true
  cds_config:
    resource_api_version: V3
    ads: {}
  lds_config:
    resource_api_version: V3
    ads: {}
static_resources:
  listeners:
  - name: default_listener
    address:
      socket_address:
        address: 0.0.0.0
        port_value: 15006
    use_original_dst: true
    filter_chains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          stat_prefix: tcp
          cluster: origin_cluster
  clusters:
  - name: xds_cluster
    connect_timeout: 2s
    type: STRICT_DNS
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: xds_cluster
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: %s
                port_value: 9002
    http2_protocol_options: {}
  - name: origin_cluster
    connect_timeout: 5s
    type: ORIGINAL_DST
    lb_policy: CLUSTER_PROVIDED
`

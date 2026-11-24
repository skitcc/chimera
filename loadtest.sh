docker run --rm -i --network host \
    -v "$(pwd)/loadtest:/loadtest" \
    grafana/k6 run /loadtest/balance.js
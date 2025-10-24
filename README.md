# Clear ALL forwarding logs
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db circuit-fwd-log

# Clear forwarding logs older than 1 week
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=1w circuit-fwd-log

# Clear logs older than 2 weeks
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=2w circuit-fwd-log

# Clear logs older than 1 month
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=1m circuit-fwd-log

# Clear logs older than 3 months
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=3m circuit-fwd-log

# Clear multiple buckets (all data)
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db \
    circuit-fwd-log \
    closed-chan-bucket \
    historical-chan-bucket

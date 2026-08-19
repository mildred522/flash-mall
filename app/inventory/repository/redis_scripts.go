package repository

const seedStockLuaScript = `
for i = 1, #KEYS do
  redis.call("set", KEYS[i], ARGV[i])
end
return 1
`

const resetReservedStockLuaScript = `
redis.call("set", KEYS[1], ARGV[1] or 0)
return 1
`

const sumStockLuaScript = `
local exists = 0
local total = 0
for i = 1, #KEYS do
  local stock = redis.call("get", KEYS[i])
  if stock then
    exists = 1
    total = total + tonumber(stock)
  end
end
return {exists, total}
`

const getReservedStockLuaScript = `
local reserved = redis.call("get", KEYS[1])
if not reserved then
  return 0
end
return tonumber(reserved)
`

const batchStockLuaScript = `
local shardCount = tonumber(ARGV[1])
local result = {}
for productOffset = 1, #ARGV - 1 do
  local productID = tonumber(ARGV[productOffset + 1])
  local keyOffset = (productOffset - 1) * (shardCount + 1)
  local exists = 0
  local available = 0
  for shardOffset = 1, shardCount do
    local stock = redis.call("get", KEYS[keyOffset + shardOffset])
    if stock then
      exists = 1
      available = available + tonumber(stock)
    end
  end
  local reserved = tonumber(redis.call("get", KEYS[keyOffset + shardCount + 1]) or "0")
  table.insert(result, productID)
  table.insert(result, exists)
  table.insert(result, available)
  table.insert(result, reserved)
end
return result
`

const adjustAvailableStockLuaScript = `
local delta = tonumber(ARGV[1])
local shardCount = tonumber(ARGV[2])
local start = tonumber(ARGV[3])
if delta > 0 then
  redis.call("incrby", KEYS[start], delta)
  return 1
end
local amount = -delta
local total = 0
for i = 1, shardCount do
  total = total + tonumber(redis.call("get", KEYS[i]) or "0")
end
if total < amount then
  return -2
end
local remain = amount
for i = 0, shardCount - 1 do
  local idx = ((start - 1 + i) % shardCount) + 1
  local stock = tonumber(redis.call("get", KEYS[idx]) or "0")
  if stock > 0 then
    local take = stock
    if take > remain then
      take = remain
    end
    redis.call("decrby", KEYS[idx], take)
    remain = remain - take
    if remain == 0 then
      return 1
    end
  end
end
return -2
`

const reserveStockLuaScript = `
local shardCount = tonumber(ARGV[3])
local reservationKey = KEYS[shardCount + 1]
local reservedKey = KEYS[shardCount + 2]
local expiryIndexKey = KEYS[shardCount + 3]
local status = redis.call("hget", reservationKey, "status")
if status then
  local existingProductID = redis.call("hget", reservationKey, "product_id")
  local existingQuantity = tonumber(redis.call("hget", reservationKey, "quantity") or "0")
  if tostring(existingProductID) ~= tostring(ARGV[5]) or existingQuantity ~= tonumber(ARGV[1]) then
    return -3
  end
  return 1
end
local amount = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
local expirySeconds = tonumber(ARGV[7])
local start = tonumber(ARGV[4])
local productID = ARGV[5]
local orderID = ARGV[6]
local hasStockKey = false
for i = 0, shardCount - 1 do
  local idx = ((start + i) % shardCount) + 1
  local stockKey = KEYS[idx]
  local stock = redis.call("get", stockKey)
  if stock then
    hasStockKey = true
    stock = tonumber(stock)
    if stock >= amount then
      redis.call("decrby", stockKey, amount)
      redis.call("incrby", reservedKey, amount)
      redis.call("hset", reservationKey, "status", "reserved", "product_id", productID, "quantity", amount, "shard_index", idx, "order_id", orderID)
      if ttl and ttl > 0 then
        redis.call("expire", reservationKey, ttl)
        local now = tonumber(redis.call("time")[1])
        redis.call("zadd", expiryIndexKey, now + expirySeconds, orderID)
      end
      return 1
    end
  end
end
if hasStockKey == false then
  return -1
end
return -2
`

const confirmDeductLuaScript = `
local reservationKey = KEYS[1]
local status = redis.call("hget", reservationKey, "status")
local changed = 0
if not status then
  return 1
end
if status == "released" then
  return 1
end
if status == "reserved" then
  changed = 1
  redis.call("hset", reservationKey, "status", "confirmed")
  local productID = redis.call("hget", reservationKey, "product_id")
  local quantity = tonumber(redis.call("hget", reservationKey, "quantity") or "0")
  if productID and quantity > 0 then
    local reservedKey = "stock_reserved:" .. productID
    local current = tonumber(redis.call("get", reservedKey) or "0")
    local nextValue = current - quantity
    if nextValue < 0 then
      nextValue = 0
    end
    redis.call("set", reservedKey, nextValue)
  end
end
local ttl = tonumber(ARGV[1])
if ttl and ttl > 0 then
  redis.call("expire", reservationKey, ttl)
end
if KEYS[2] and ARGV[3] then
  redis.call("zrem", KEYS[2], ARGV[3])
end
local finalDeductEnabled = tonumber(ARGV[2])
local mysqlDeducted = redis.call("hget", reservationKey, "mysql_deducted")
if finalDeductEnabled == 1 and mysqlDeducted ~= "1" then
  return 2
end
if changed == 1 then
  return 3
end
return 1
`

const markMySQLDeductedLuaScript = `
local reservationKey = KEYS[1]
redis.call("hset", reservationKey, "mysql_deducted", "1")
local ttl = tonumber(ARGV[1])
if ttl and ttl > 0 then
  redis.call("expire", reservationKey, ttl)
end
return 1
`

const getReservationLuaScript = `
local reservationKey = KEYS[1]
local productID = redis.call("hget", reservationKey, "product_id")
local quantity = redis.call("hget", reservationKey, "quantity")
if not productID or not quantity then
  return {0, 0, 0}
end
local shardIndex = redis.call("hget", reservationKey, "shard_index") or "0"
return {tonumber(productID), tonumber(quantity), tonumber(shardIndex)}
`

const hydrateReservationLuaScript = `
if redis.call("hget", KEYS[1], "status") then
  return 1
end
redis.call("hset", KEYS[1],
  "status", ARGV[1],
  "product_id", ARGV[2],
  "quantity", ARGV[3],
  "shard_index", ARGV[4],
  "order_id", ARGV[5])
local ttl = tonumber(ARGV[6])
if ttl and ttl > 0 then
  redis.call("expire", KEYS[1], ttl)
end
return 1
`

const restoreProductStockLuaScript = `
local shardCount = tonumber(ARGV[1])
for i = 1, shardCount do
  redis.call("set", KEYS[i], ARGV[i + 1])
end
redis.call("set", KEYS[shardCount + 1], ARGV[shardCount + 2])
return 1
`

const resetReservationRecoveryIndexesLuaScript = `
for i = 1, #KEYS do
  redis.call("del", KEYS[i])
end
return 1
`

const restoreReservationLuaScript = `
local status = ARGV[1]
local orderID = ARGV[5]
redis.call("del", KEYS[1])
redis.call("hset", KEYS[1],
  "status", status,
  "product_id", ARGV[2],
  "quantity", ARGV[3],
  "shard_index", ARGV[4],
  "order_id", orderID)
if status == "confirmed" then
  redis.call("hset", KEYS[1], "mysql_deducted", "1")
  redis.call("expire", KEYS[1], ARGV[7])
  redis.call("zrem", KEYS[2], orderID)
  redis.call("hdel", KEYS[4], orderID)
  redis.call("zrem", KEYS[5], orderID)
else
  redis.call("expire", KEYS[1], ARGV[6])
  if not redis.call("zscore", KEYS[5], orderID) then
    redis.call("zadd", KEYS[2], ARGV[8], orderID)
  end
end
redis.call("zrem", KEYS[3], orderID)
return 1
`

const releaseStockLuaScript = `
local shardCount = tonumber(ARGV[2])
local reservationKey = KEYS[shardCount + 1]
local reservedKey = KEYS[shardCount + 2]
local expiryIndexKey = KEYS[shardCount + 3]
local status = redis.call("hget", reservationKey, "status")
if not status or status == "released" then
  return 1
end
local quantity = tonumber(redis.call("hget", reservationKey, "quantity"))
local shardIndex = tonumber(redis.call("hget", reservationKey, "shard_index"))
local stockKey = KEYS[shardIndex]
if stockKey and quantity and quantity > 0 then
  redis.call("incrby", stockKey, quantity)
end
if status == "reserved" and quantity and quantity > 0 then
  local current = tonumber(redis.call("get", reservedKey) or "0")
  local nextValue = current - quantity
  if nextValue < 0 then
    nextValue = 0
  end
  redis.call("set", reservedKey, nextValue)
end
redis.call("hset", reservationKey, "status", "released")
local orderID = redis.call("hget", reservationKey, "order_id")
if expiryIndexKey and orderID then
  redis.call("zrem", expiryIndexKey, orderID)
end
local ttl = tonumber(ARGV[1])
if ttl and ttl > 0 then
  redis.call("expire", reservationKey, ttl)
end
return 1
`

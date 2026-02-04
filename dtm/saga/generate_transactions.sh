#!/bin/bash

echo "Gerando transações de teste..."

for i in {1..50}; do
  QUANTITY=$((RANDOM % 10 + 1))
  AMOUNT=$((RANDOM % 500 + 100))
  
  curl -s -X POST http://localhost:8081/api/orders \
    -H "Content-Type: application/json" \
    -d "{\"user_id\":\"USER-$i\",\"product_id\":\"PROD-001\",\"quantity\":$QUANTITY,\"amount\":$AMOUNT}" \
    > /dev/null
  
  echo -n "."
  sleep 0.2
done

echo ""
echo "✅ 50 transações criadas!"

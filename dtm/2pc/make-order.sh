#!/bin/bash

# Script simples para fazer um pedido com UUIDs fixos
# 2PC/XA implementation

# UUIDs FIXOS (sempre os mesmos!)
PRODUCT_ID="550e8400-e29b-41d4-a716-446655440001"  # Product 00001
USER_ID="6ba7b810-9dad-11d1-80b4-00c04fd43001"     # Primeiro usuário

# Configurações do pedido (sempre 1 unidade)
TOTAL_PRICE=${1:-1000}      # Preço total (padrão: 1000)

echo "🛒 Fazendo pedido com 2PC/XA e UUIDs fixos (1 unidade)..."
echo "   Produto: $PRODUCT_ID"
echo "   Usuário: $USER_ID"
echo "   Quantidade: 1 unidade"
echo "   Preço Total: $TOTAL_PRICE"
echo ""

# Faz o pedido (sempre 1 unidade) - XA é síncrono
response=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8086/api/orders \
    -H "Content-Type: application/json" \
    -d "{
        \"user_id\": \"${USER_ID}\",
        \"product_id\": \"${PRODUCT_ID}\",
        \"total_price\": ${TOTAL_PRICE}
    }")

http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "${http_code}" == "200" ]; then
    echo "✅ PEDIDO CRIADO COM SUCESSO (200 OK)!"
    echo "   Transação XA (2PC) completada sincronamente"
    echo ""
    echo "Response: ${body}"
    echo ""
    
    # Extrair orderID e traceID da resposta
    order_id=$(echo "$body" | grep -o '"order_id":"[^"]*"' | cut -d'"' -f4)
    trace_id=$(echo "$body" | grep -o '"trace_id":"[^"]*"' | cut -d'"' -f4)
    
    if [ -n "$order_id" ]; then
        echo "📦 Order ID: ${order_id}"
        echo "🔍 Trace ID: ${trace_id}"
        echo ""
        echo "💡 O pedido foi completado sincronamente com 2PC/XA"
        echo "   Status: completed"
        echo "   Consulte o banco para verificar:"
        echo "   SELECT * FROM orders WHERE order_id = '${order_id}';"
    fi
else
    echo "❌ ERRO AO CRIAR PEDIDO (${http_code})"
    echo "   Transação XA (2PC) falhou"
    echo ""
    echo "Response: ${body}"
fi

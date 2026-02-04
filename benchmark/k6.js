import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = 'http://localhost:8081/api/orders';

export const options = {
  scenarios: {
    teste_vazao_fixa: {
      executor: 'ramping-arrival-rate',
      
      startRate: 5,           
      timeUnit: '1s',         
      preAllocatedVUs: 10,    
      maxVUs: 15,             
      
      stages: [
        { duration: '170s', target: 5 },
        { duration: '10s', target: 0 }, 
      ],
      
      exec: 'createOrder',
    },
  },
};

// Funções utilitárias para gerar dados aleatórios
function getRandomInt(min, max) {
  min = Math.ceil(min);
  max = Math.floor(max);
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

// A função principal que será executada em loop
export function createOrder() {
  // Gera dados aleatórios
  const quantity = 1;
  const amount = 1;
  const userId = `6ba7b810-9dad-11d1-80b4-00c04fd43001`; 

  const payload = JSON.stringify({
    user_id: userId,
    product_id: '550e8400-e29b-41d4-a716-446655440001',
    quantity: quantity,
    amount: amount,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  // Requisição HTTP POST
  const res = http.post(BASE_URL, payload, params);

  // Verificação (Check)
  check(res, {
    'status is 200 or 202': (r) => r.status === 200 || r.status === 202,
  });
}

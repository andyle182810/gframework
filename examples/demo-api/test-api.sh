#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

API_URL="http://localhost:8080"
METRICS_URL="http://localhost:9090"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Testing Demo API${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Generate UUID for requests
REQUEST_ID=$(uuidgen)

# Test 1: Health Check
echo -e "${GREEN}1. Testing Health Check${NC}"
curl -s -X GET "$API_URL/health" | jq '.'
echo -e "\n"

# Test 2: Readiness Check
echo -e "${GREEN}2. Testing Readiness Check${NC}"
curl -s -X GET "$API_URL/ready" | jq '.'
echo -e "\n"

# Test 3: Create User
echo -e "${GREEN}3. Creating a new user${NC}"
RESPONSE=$(curl -s -X POST "$API_URL/v1/users" \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: $(uuidgen)" \
  -d '{
    "name": "Test User",
    "email": "test-'$(date +%s)'@example.com"
  }')
echo "$RESPONSE" | jq '.'
USER_ID=$(echo "$RESPONSE" | jq -r '.data.id')
echo -e "\n"

# Test 4: Get User
echo -e "${GREEN}4. Getting user by ID: $USER_ID${NC}"
curl -s -X GET "$API_URL/v1/users/$USER_ID" \
  -H "X-Request-ID: $(uuidgen)" | jq '.'
echo -e "\n"

# Test 5: Update User
echo -e "${GREEN}5. Updating user: $USER_ID${NC}"
curl -s -X PATCH "$API_URL/v1/users/$USER_ID" \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: $(uuidgen)" \
  -d '{
    "name": "Updated Test User",
    "email": "updated-'$(date +%s)'@example.com"
  }' | jq '.'
echo -e "\n"

# Test 6: List Users
echo -e "${GREEN}6. Listing users (page 1, 5 items)${NC}"
curl -s -X GET "$API_URL/v1/users?page=1&pageSize=5" \
  -H "X-Request-ID: $(uuidgen)" | jq '.'
echo -e "\n"

# Test 7: Metrics
echo -e "${GREEN}7. Checking metrics endpoint${NC}"
curl -s "$METRICS_URL/metrics" | head -n 20
echo -e "\n..."

# Test 8: Metrics Status
echo -e "${GREEN}8. Checking metrics status${NC}"
curl -s "$METRICS_URL/status" | jq '.'
echo -e "\n"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}All tests completed!${NC}"
echo -e "${BLUE}========================================${NC}"

#!/bin/bash
set -euo pipefail

# === CONFIGURAÇÃO ===
IMAGE_NAME="joelgarciajr84/rinha-backend-go"
PLATFORM="linux/amd64"

# === OBTENDO COMMIT ===
GIT_SHA=$(git rev-parse --short HEAD)
TAG="rinha-galo-${GIT_SHA}"

echo ">> Buildando imagem para ${IMAGE_NAME}:${TAG} (${PLATFORM})"

# === BUILD COM COMMIT EMBUTIDO ===
docker buildx build \
  --platform="${PLATFORM}" \
  --build-arg BUILD_COMMIT="${GIT_SHA}" \
  -t "${IMAGE_NAME}:${TAG}" \
  -t "${IMAGE_NAME}:latest" \
  .

# === PUBLICAÇÃO ===
echo ">> Fazendo push para o Docker Hub..."
docker push "${IMAGE_NAME}:${TAG}"
docker push "${IMAGE_NAME}:latest"

# === MOSTRANDO DIGEST ===
DIGEST=$(docker buildx imagetools inspect "${IMAGE_NAME}:${TAG}" | grep 'Digest:' | awk '{print $2}')
echo ">> Publicado com sucesso!"
echo "   Tag:    ${TAG}"
echo "   Digest: ${DIGEST}"

# === SUGESTÃO PARA O COMPOSE ===
echo
echo ">> Para usar no docker-compose.yml com digest fixo:"
echo "image: ${IMAGE_NAME}@${DIGEST}"

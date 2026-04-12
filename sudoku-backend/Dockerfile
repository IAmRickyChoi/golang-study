# 1. 빌드를 위한 베이스 이미지 설정 (ベースイメージ)
FROM golang:alpine AS builder

# 2. 작업 디렉토리 설정 (作業ディレクトリ)
WORKDIR /app

# 의존성 먼저 캐시 (코드 변경 시 재다운로드 방지)
COPY go.mod go.sum ./
RUN go mod download

# 소스 복사 후 빌드
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# ─── 2단계: 실행 이미지 (최소 크기) ────────────
FROM alpine:latest

WORKDIR /root/

# 빌드된 바이너리만 복사
COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]

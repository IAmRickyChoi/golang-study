# 1. 빌드를 위한 베이스 이미지 설정 (ベースイメージ)
FROM golang:alpine AS builder

# 2. 작업 디렉토리 설정 (作業ディレクトリ)
WORKDIR /app

# 3. 모듈 파일 복사 및 의존성 다운로드
COPY go.mod go.sum ./
RUN go mod download

# 4. 소스 코드 복사 및 빌드 (ビルド)
COPY . .
# 리눅스 환경에 맞게 실행 파일을 컴파일합니다.
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# 5. 실행을 위한 가벼운 최종 이미지 생성 (マルチステージビルド)
FROM alpine:latest
WORKDIR /root/

# 6. 빌드된 실행 파일만 가져오기
COPY --from=builder /app/main .

# 7. 포트 노출 (ポート公開)
EXPOSE 8080

# 8. 서버 실행 (実行コマンド)
CMD ["./main"]
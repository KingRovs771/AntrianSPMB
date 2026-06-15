# Dockerfile - Single Stage Offline Build using precompiled host binary
# Gunakan nginx:alpine sebagai base karena sudah tersedia di lokal
FROM nginx:alpine

WORKDIR /app

# Copy binary yang sudah di-build di host
COPY main .

# Copy template dan asset statis
COPY views ./views
COPY public ./public
COPY .env ./.env

# Expose port aplikasi
EXPOSE 3000

# Jalankan binary Go
CMD ["./main"]

FROM alpine:latest
WORKDIR /app
# binarni fajl gradi start.sh lokalno (sa opcionalnim UPX) pre docker build
COPY ntech .
EXPOSE 8000
CMD ["./ntech"]

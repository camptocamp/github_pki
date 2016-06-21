FROM scratch
ADD ca-certificates.crt /etc/ssl/certs/
ADD github_pki /
ENTRYPOINT ["/github_pki"]
CMD [""]

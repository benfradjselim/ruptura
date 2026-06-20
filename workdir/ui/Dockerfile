FROM nginx:1.27-alpine

# Static files are pre-built by Vite into workdir/web/ (outDir: '../web').
# The Docker build context must be ./workdir so this COPY path is reachable.
COPY web /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]

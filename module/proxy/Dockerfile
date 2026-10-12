FROM nginx:latest
EXPOSE 80 443

RUN echo "load_module modules/ngx_http_xslt_filter_module.so;" "$(cat /etc/nginx/nginx.conf)" > /etc/nginx/nginx.conf

COPY nginx.conf /etc/nginx/templates/default.conf.template
COPY xslt /etc/nginx/xslt
COPY static /var/www/static
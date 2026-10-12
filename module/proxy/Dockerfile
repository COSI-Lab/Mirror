FROM nginx:latest
EXPOSE 80 443

# Enable the XSLT module
RUN echo "load_module modules/ngx_http_xslt_filter_module.so;" "$(cat /etc/nginx/nginx.conf)" > /etc/nginx/nginx.conf

# Copy config file
COPY site.conf /etc/nginx/templates/default.conf.template

# Copy in static page contents
COPY xslt /etc/nginx/xslt
COPY static /var/www/static
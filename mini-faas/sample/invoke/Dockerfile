FROM debian:stable-slim

ARG sample_dir=/aliyuncnpc/sample
WORKDIR ${sample_dir}

COPY ./sample-invoke ${sample_dir}/

ENTRYPOINT ["/aliyuncnpc/sample/sample-invoke"]

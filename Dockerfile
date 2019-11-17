FROM scratch

COPY slowjoe /

ENTRYPOINT [ "/slowjoe" ]

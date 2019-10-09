FROM scratch

COPY build/_output/bin/multicloud-operators-deployable .
CMD ["./multicloud-operators-deployable"]

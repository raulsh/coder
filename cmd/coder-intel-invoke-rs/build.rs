fn main() {
    prost_build::compile_protos(
        &["../../inteld/proto/inteld.proto"],
        &["../../inteld/proto"],
    )
    .unwrap();
}

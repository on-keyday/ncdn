use edge_api::api::{self, edge::DiffKind};
// 1. ホストからデータを受け取るためのバッファを確保
const BUF_SIZE: usize = 2048;

#[no_mangle]
pub extern "C" fn on_request() {
    api::init_logger(log::LevelFilter::Info);

    let mut buffer = vec![0u8; BUF_SIZE];

    let req = match api::get_request(&mut buffer) {
        Ok(req_info) => {
            req_info
        }
        Err(err) => {
            log::error!("[Acme] Failed to decode request info: {}", err);
            return;
        }
    };

    const REQ_TOKEN : &str = "request_token"; 

    if req.path() == format!("/.well-known/acme-challenge/{}", REQ_TOKEN) {
        const TOKEN: &str = "token.thumbprint"; // Replace with actual token logic
        api::ChangeSet::new()
            .request_routing(api::Routing::deny)// not to go origin
            .status(200)
            .response_field(DiffKind::replace, "Content-Type", "text/plain")
            .response_body(TOKEN.as_bytes())
            .apply().expect("Failed to create response changes");
        log::info!("[Acme] Responding with token");
    }
}

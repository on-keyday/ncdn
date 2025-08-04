

mod api;

// 1. ホストからデータを受け取るためのバッファを確保
const BUF_SIZE: usize = 2048;

#[no_mangle]
pub extern "C" fn on_request() {
    api::init_logger(log::LevelFilter::Info);
    log::info!("[Wasm] on_request hook called");


    let mut buffer = vec![0u8; BUF_SIZE];


    let req = match api::get_request(&mut buffer){
        Ok(req_info) => {
            log::info!("[Wasm] sizeof RequestInfo: {}", std::mem::size_of::<api::Request>());
            log::info!("[Wasm] serialized RequestInfo size: {}", req_info.request_size());
            log::info!("[Wasm] Successfully decoded RequestInfo");
            log::info!("[Wasm] Method: {}", req_info.method());
            // 4. デコードしたデータを使って何らかの処理を行う
            log::info!("[Wasm] Path: {}", req_info.path());
            for (key,value) in req_info.headers() {
                log::info!("[Wasm] Header: {} = {}", key, value);
            }
            req_info
        }
        Err(err) => {
            log::error!("[Wasm] Failed to decode request info: {}", err);
            return;
        }
    };

    if req.path() == "/" {
        let redirect_url = format!("https://{}/index.html", req.host());
        api::ChangeSet::new()
            .routing(api::Routing::deny) 
            .status(308) // HTTP 308 Permanent Redirect
            .location(&redirect_url)
            .apply().expect("Failed to create redirect changes");
        log::info!("[Wasm] Redirecting to: {}", redirect_url);
        // anyway, we need to save the buffer
    }
 
    let req_size = req.request_size();
    buffer.resize(req_size, 0);
    api::save_buffer(buffer);

}

#[no_mangle]
pub extern "C" fn on_response() {
    let buffer = api::get_buffer();
    if buffer.is_none() {
        log::warn!("[Wasm] No buffer available");
        return;
    }
    let mut buffer = buffer.unwrap();
    log::info!("[Wasm] on_response hook called");
    log::info!("[Wasm] Buffer size: {}", buffer.len());
    match api::decode_request(&buffer) {
        Ok(req_info) => {
            log::info!("[Wasm] Protocol: {} {}",req_info.protocol(),if req_info.is_tls() { "(TLS)" } else { "(Plain)" });
            req_info
        }
        Err(err) => {
            log::error!("[Wasm] Failed to decode request info: {}", err);
            return;
        }
    };

    buffer.resize(BUF_SIZE, 0);

    match api::get_response(&mut buffer) {
        Ok(resp_info) => {
            log::info!("[Wasm] sizeof ResponseInfo: {}", std::mem::size_of::<api::Response>());
            log::info!("[Wasm] serialized ResponseInfo size: {}", resp_info.response_size());
            log::info!("[Wasm] Successfully decoded ResponseInfo");
            log::info!("[Wasm] Status: {}", resp_info.status());
            for (key, value) in resp_info.headers() {
                log::info!("[Wasm] Header: {} = {}", key, value);
            }
        }
        Err(err) => {
            log::error!("[Wasm] Failed to decode response info: {}", err);
        }
    }

    api::ChangeSet::new()
        .response_field(api::DiffKind::replace, "X-Wasm-Processed", "true")
        .apply()
        .expect("Failed to apply response changes");
}
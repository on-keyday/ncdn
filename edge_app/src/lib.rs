

mod api;



#[no_mangle]
pub extern "C" fn on_request() {
    api::init_logger(log::LevelFilter::Info);
    log::info!("[Wasm] on_request hook called");

    // 1. ホストからデータを受け取るためのバッファを確保
    const BUF_SIZE: usize = 2048;
    let mut buffer = vec![0u8; BUF_SIZE];

    let req = api::get_request(&mut buffer);

    let req_size = match req {
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
            req_info.request_size()
        }
        Err(err) => {
            log::error!("[Wasm] Failed to decode request info: {}", err);
            0
        }
    };

    buffer.resize(req_size, 0);
    api::save_buffer(buffer);
}

#[no_mangle]
pub extern "C" fn on_response() {
    let buffer = api::get_buffer().unwrap();
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
}
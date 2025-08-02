use log::Log;

use crate::edge::LogLevel;

mod edge;


#[link(wasm_import_module = "ncdn")]
extern "C" {
    // get_request_info(pointer: u32, size: u32) -> u32
    fn get_request_info(pointer: *mut u8, size: u32) -> u32;
    fn log_output(level: u32, pointer: *const u8, size: u32) -> u32;
    fn save_buffer_pointer(pointer: *const (), size: u32);
    fn get_buffer_pointer(pointer: *mut *const u8,size: *mut u32);

    // get_response_info(pointer: u32, size: u32) -> u32
    //fn get_response_info(pointer: *mut u8, size: u32) -> u32;
}

// 1. 独自のロガー構造体を定義
struct CustomLogger;

// 2. `log::Log` トレイトを実装して、カスタム処理を記述
impl log::Log for CustomLogger {
    fn enabled(&self, metadata: &log::Metadata) -> bool {
        // ここではInfoレベル以上のログを有効にする
        metadata.level() <= log::Level::Info
    }

    fn log(&self, record: &log::Record) {
        if self.enabled(record.metadata()) {
            // ★★★ ここがカスタム関数にあたる部分 ★★★
            // 本来はGUIへの描画やファイル書き込みなどを行う
            let message = format!("[edge_app] {}: {}", record.level(), record.args());
            let level = match record.level() {
                log::Level::Debug => LogLevel::DEBUG,
                log::Level::Info => LogLevel::INFO,
                log::Level::Warn => LogLevel::WARN,
                log::Level::Error => LogLevel::ERROR,
                log::Level::Trace => LogLevel::TRACE,
            };
            unsafe {
                let level_value = level as u32;
                log_output(level_value, message.as_ptr(), message.len() as u32);
            }
        }
    }

    fn flush(&self) {}
}


#[no_mangle]
pub extern "C" fn on_request() {
    log::set_logger(&CustomLogger).expect("Failed to set logger");
    log::set_max_level(log::LevelFilter::Info);
    log::info!("[Wasm] on_request hook called");

    // 1. ホストからデータを受け取るためのバッファを確保
    const BUF_SIZE: usize = 2048;
    let mut buffer = vec![0u8; BUF_SIZE];

    // 2. ホスト関数を呼び出し、バッファにリクエスト情報を書き込んでもらう
    let written_size = unsafe {
        get_request_info(buffer.as_mut_ptr(), BUF_SIZE as u32)
    };

    if written_size == 0 {
        log::error!("[Wasm] Failed to get request info from host.");
        return;
    }

    match edge::RequestInfo::decode_exact(&buffer[..written_size as usize]) {
        Ok(req_info) => {
            //log::info!("[Wasm] Successfully decoded RequestInfo");
            let method = if req_info.method.method == edge::Method::OTHER {
                unsafe {
                    String::from_utf8_unchecked(req_info.method.method_name().unwrap().clone())
                }
            } else {
                format!("{}", req_info.method.method)
            };
            log::info!("[Wasm] Method: {}", method);
            // 4. デコードしたデータを使って何らかの処理を行う
            log::info!("[Wasm] Path: {}",unsafe {String::from_utf8_unchecked(req_info.path.path.data)});
            for field in req_info.header.fields {
                log::info!("[Wasm] Header: {} = {}", 
                    unsafe { String::from_utf8_unchecked(field.key.data) },
                    unsafe { String::from_utf8_unchecked(field.value.data) }
                );

            }
        }
        Err(e) => {
            log::error!("[Wasm] Failed to decode request info: {}", e);
        }
    }
    buffer.resize(written_size as usize, 0);
    buffer.shrink_to_fit();
    unsafe { save_buffer_pointer(buffer.as_ptr() as *const (), written_size as u32) };
    buffer.leak(); // バッファをリークして、ホストに渡す
}

#[no_mangle]
pub extern "C" fn on_response() {
    let mut pointer: *const u8 = std::ptr::null();
    let mut size: u32 = 0;
    unsafe {
        get_buffer_pointer(&mut pointer, &mut size)
    };
    if pointer.is_null() || size == 0 {
        log::error!("[Wasm] No request info available.");
        return; 
    }
    let buffer = unsafe { std::slice::from_raw_parts(pointer as *const u8, size as usize) };
    match edge::RequestInfo::decode_exact(buffer) {
        Ok(req_info) => {
            log::info!("[Wasm] Successfully decoded RequestInfo for response");
            log::info!("[Wasm] Protocol: {}", req_info.protocol.protocol);
        },
        Err(e) => {
            log::error!("[Wasm] Failed to decode request info for response: {}", e);
        }
    }
}
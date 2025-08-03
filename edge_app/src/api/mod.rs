pub mod edge;


#[link(wasm_import_module = "ncdn")]
extern "C" {
    // get_request_info(pointer: u32, size: u32) -> u32
    fn get_request_info(pointer: *mut u8, size: u32) -> u32;
    fn log_output(level: u32, pointer: *const u8, size: u32) -> u32;
    fn save_buffer_pointer(pointer: *const (), size: u32);
    fn get_buffer_pointer(pointer: *mut *const u8,size: *mut u32);

    // get_response_info(pointer: u32, size: u32) -> u32
    fn get_response_info(pointer: *mut u8, size: u32) -> u32;
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
                log::Level::Debug => edge::LogLevel::DEBUG,
                log::Level::Info => edge::LogLevel::INFO,
                log::Level::Warn => edge::LogLevel::WARN,
                log::Level::Error => edge::LogLevel::ERROR,
                log::Level::Trace => edge::LogLevel::TRACE,
            };
            unsafe {
                let level_value :u8 = level.into();
                log_output(level_value as u32, message.as_ptr(), message.len() as u32);
            }
        }
    }

    fn flush(&self) {}
}

pub fn init_logger(level: log::LevelFilter) {
    // 3. ロガーを初期化
    log::set_boxed_logger(Box::new(CustomLogger)).expect("Failed to set custom logger");
    log::set_max_level(level);
}

pub fn save_buffer(mut buffer: Vec<u8>) {
    if buffer.is_empty() {
        log::warn!("[Wasm] Attempted to set an empty buffer");
        return;
    }
    buffer.shrink_to_fit(); // 不要な容量を削除
    let buffer = buffer.leak();
    unsafe {
        save_buffer_pointer(buffer.as_ptr() as *const (), buffer.len() as u32);
    }
}

pub fn get_buffer() -> Option<Vec<u8>> {
    let mut pointer: *const u8 = std::ptr::null();
    let mut size: u32 = 0;
    unsafe {
        get_buffer_pointer(&mut pointer, &mut size);
    }
    if pointer.is_null() || size == 0 {
        None
    } else {
        Some(unsafe {Vec::from_raw_parts(pointer as *mut u8, size as usize, size as usize)})
    }
}

pub struct Request<'a> {
    info :edge::RequestInfo<'a>,
    request_size: usize,
}

fn loosy_or_unknown(data: &[u8]) -> &str {
    match String::from_utf8_lossy(data) {
        std::borrow::Cow::Borrowed(s) => s,
        _ => "UNKNOWN",
    }
}

impl Request<'_> {
    pub fn method(&self) -> &str {
        if self.info.method.method == edge::Method::OTHER {
            loosy_or_unknown(self.info.method.method_name().unwrap())
        } else {
            let method: Option<&str> = self.info.method.method.into();
            method.unwrap_or("UNKNOWN")
        }
    }

    pub fn path(&self) -> &str {
        loosy_or_unknown(&self.info.path.path.data)
    }

    pub fn headers(&self) -> impl std::iter::Iterator<Item = (&str, &str)> + '_ {
        self.info.header.fields.iter().map(|field| {
            (
                loosy_or_unknown(&field.key.data),
                loosy_or_unknown(&field.value.data),
            )
        })
    }

    pub fn protocol(&self) -> &str {
        match self.info.protocol.protocol {
            edge::Protocol::http1_plain | edge::Protocol::http1 => "HTTP/1.1",
            edge::Protocol::h2 => "HTTP/2",
            edge::Protocol::h3 => "HTTP/3",
            edge::Protocol::other => loosy_or_unknown(&self.info.protocol.protocol_name().unwrap()),
            _ => "UNKNOWN",
        }
    }

    pub fn remote_addr(&self) -> (std::net::IpAddr,u16) {
        if self.info.remoteAddr.is_v6() {
            ((*self.info.remoteAddr.addr_v6().unwrap_or_else(||&[0;16])).into(), self.info.remotePort)
        } else {
            ((*self.info.remoteAddr.addr_v4().unwrap_or_else(||&[0;4])).into(), self.info.remotePort)
        }
    }

    pub fn request_size(&self) -> usize {
        self.request_size
    }

    pub fn is_tls(&self) -> bool {
        self.info.protocol.protocol == edge::Protocol::http1 ||
        self.info.protocol.protocol == edge::Protocol::h2 || 
        self.info.protocol.protocol == edge::Protocol::h3
    }

}

pub struct Response<'a> {
    info: edge::ResponseInfo<'a>,
    response_size: usize,
}

impl Response<'_> {
    pub fn status(&self) -> u16 {
        self.info.status
    }

    pub fn headers(&self) -> impl std::iter::Iterator<Item = (&str, &str)> + '_ {
        self.info.header.fields.iter().map(|field| {
            (
                loosy_or_unknown(&field.key.data),
                loosy_or_unknown(&field.value.data),
            )
        })
    }

    pub fn response_size(&self) -> usize {
        self.response_size
    }
}

#[derive(Debug)]
pub enum Error {
    NotEnoughBuffer,
    DecodeError(edge::Error),
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::NotEnoughBuffer => write!(f, "Not enough buffer space"),
            Error::DecodeError(e) => write!(f, "Decode error: {}", e),
        }
    }
}   

impl std::error::Error for Error {}

pub fn decode_request<'a>(buffer: &'a [u8]) -> Result<Request<'a>, Error> {
    match edge::RequestInfo::decode_exact_direct(buffer) {
        Ok(req_info) => {
            Ok(Request { info: req_info, request_size: buffer.len() })
        }
        Err(e) => {
            Err(Error::DecodeError(e))
        }
    }
}

pub fn decode_response<'a>(buffer: &'a [u8]) -> Result<Response<'a>, Error> {
    match edge::ResponseInfo::decode_exact_direct(buffer) {
        Ok(resp_info) => {
            Ok(Response { info: resp_info, response_size: buffer.len() })
        }
        Err(e) => {
            Err(Error::DecodeError(e))
        }
    }
}

pub fn get_request<'a>(buffer :&'a mut [u8]) -> Result<Request<'a>,Error> {
    let written_size = unsafe {
        get_request_info(buffer.as_ptr() as *mut u8, buffer.len() as u32)
    };
    if written_size == 0 {
        return Err(Error::NotEnoughBuffer);
    }

    decode_request(&buffer[..written_size as usize])
}

pub fn get_response<'a>(buffer: &'a mut [u8]) -> Result<Response<'a>, Error> {
    let written_size = unsafe {
        get_response_info(buffer.as_ptr() as *mut u8, buffer.len() as u32)
    };
    if written_size == 0 {
        return Err(Error::NotEnoughBuffer);
    }

    decode_response(&buffer[..written_size as usize])
}

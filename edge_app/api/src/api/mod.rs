pub mod edge;


#[link(wasm_import_module = "ncdn")]
extern "C" {
    // get_request_info(pointer: u32, size: u32) -> u32
    fn get_request_info(pointer: *mut u8, size: u32) -> u32;
    // get_response_info(pointer: u32, size: u32) -> u32
    fn get_response_info(pointer: *mut u8, size: u32) -> u32;
    fn log_output(level: u32, pointer: *const u8, size: u32) -> u32;
    fn save_buffer_pointer(pointer: *const (), size: u32);
    fn get_buffer_pointer(pointer: *mut *const u8,size: *mut u32);

    fn change_request_info(pointer: *const u8, size: u32) -> u32;
    fn change_response_info(pointer: *const u8, size: u32) -> u32;
    
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
        unsafe {
            save_buffer_pointer(std::ptr::null(), 0); // バッファをクリア
        }
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

pub type Routing = edge::Routing;
pub type DiffKind = edge::DiffKind;

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

    pub fn host(&self) -> &str {
        self.headers()
            .find(|(key, _)| key.eq_ignore_ascii_case("host"))
            .map_or("UNKNOWN", |(_, value)| value)
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
    ApplyChangeFailed(bool),
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::NotEnoughBuffer => write!(f, "Not enough buffer space"),
            Error::DecodeError(e) => write!(f, "Decode error: {}", e),
            Error::ApplyChangeFailed(is_request) => write!(f, "Failed to apply changes to {}", if *is_request { "request" } else { "response" }),
        }
    }
}   

impl std::error::Error for Error {}

impl From<edge::Error> for Error {
    fn from(e: edge::Error) -> Self {
        Error::DecodeError(e)
    }
}

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

pub fn get_request_buffer<'a>(buffer: &'a mut [u8]) -> Result<&'a [u8], Error> {
    let written_size = unsafe {
        get_request_info(buffer.as_mut_ptr(), buffer.len() as u32)
    };
    if written_size == 0 {
        return Err(Error::NotEnoughBuffer);
    }
    Ok(&buffer[..written_size as usize])
}

pub fn get_request<'a>(buffer :&'a mut [u8]) -> Result<Request<'a>,Error> {
    let buffer = get_request_buffer(buffer)?;
    decode_request(&buffer)
}

pub fn get_response_buffer<'a>(buffer: &'a mut [u8]) -> Result<&'a [u8], Error> {
    let written_size = unsafe {
        get_response_info(buffer.as_mut_ptr(), buffer.len() as u32)
    };
    if written_size == 0 {
        return Err(Error::NotEnoughBuffer);
    }
    Ok(&buffer[..written_size as usize])
}

pub fn get_response<'a>(buffer: &'a mut [u8]) -> Result<Response<'a>, Error> {
    let buffer = get_response_buffer(buffer)?;
    decode_response(&buffer)
}


pub struct ChangeSet<'a> {
    request_changes: edge::ChangeSet<'a>,
    response_changes: edge::ChangeSet<'a>,
}

pub fn make_field<'a>(key: &'a str, value: &'a str) -> edge::Field<'a> {
    let mut field = edge::Field::default();
    field.key.set_data(std::borrow::Cow::Borrowed(key.as_bytes())).unwrap();
    field.value.set_data(std::borrow::Cow::Borrowed(value.as_bytes())).unwrap();
    field
}

pub fn make_header<'a,T : Iterator<Item = (&'a str, &'a str)>>(iter:T) -> edge::Header<'a> {
    let mut header = edge::Header::default();
    header.set_fields(iter.map(|(key,value)| make_field(key,value)).collect()).unwrap(); 
    header
}

pub fn make_body<'a>(data: &'a [u8]) -> edge::Body<'a> {
    let mut body = edge::Body::default();
    body.offset = 0;
    body.set_body(std::borrow::Cow::Borrowed(data)).unwrap();
    body
}

impl<'a> ChangeSet<'a> {
    pub fn new() -> Self {
        ChangeSet { request_changes: edge::ChangeSet::default(), response_changes: edge::ChangeSet::default() }
    }

    pub fn add_request_diff(&mut self, diff: edge::DiffData<'a>) -> &mut Self {
        self.request_changes.len += 1;
        self.request_changes.diff.to_mut().push(diff);
        self
    }

    pub fn add_response_diff(&mut self, diff: edge::DiffData<'a>) -> &mut Self {
        self.response_changes.len += 1;
        self.response_changes.diff.to_mut().push(diff);
        self
    }

    pub fn request_routing(&mut self, routing: edge::Routing) -> &mut Self {
       let mut r = edge::DiffData::default();
       r.diff_type = edge::DiffDataType::routing;
       r.set_routing(routing).unwrap();
       self.add_request_diff(r)
    }

    pub fn response_routing(&mut self, routing: edge::Routing) -> &mut Self {
        let mut r = edge::DiffData::default();
        r.diff_type = edge::DiffDataType::routing;
        r.set_routing(routing).unwrap();
        self.add_response_diff(r)
    }

    pub fn path_info(&mut self, path: edge::PathInfo<'a>) -> &mut Self {
        let mut p = edge::DiffData::default();
        p.diff_type = edge::DiffDataType::path;
        p.set_path(path).unwrap();
        self.add_request_diff(p)
    }

    pub fn path(&mut self, path: &'a str) -> &mut Self {
        let mut pinfo = edge::PathInfo::default();
        pinfo.path.set_data(std::borrow::Cow::Borrowed(path.as_bytes())).unwrap();
        self.path_info(pinfo)
    }

    pub fn method_info(&mut self, method: edge::MethodInfo<'a>) -> &mut Self {
        let mut m = edge::DiffData::default();
        m.diff_type = edge::DiffDataType::method;
        m.set_method(method).unwrap();
        self.add_request_diff(m)
    }

    pub fn method(&mut self, method: edge::Method) -> &mut Self {
        let mut minfo = edge::MethodInfo::default();
        minfo.method = method;
        self.method_info(minfo)
    }

    pub fn request_field_info(&mut self, op: edge::DiffKind, field: edge::Field<'a>) -> &mut Self {
        let mut diff = edge::DiffData::default();
        diff.diff_type = edge::DiffDataType::field;
        diff.kind = op;
        diff.set_field(field).unwrap();
        self.add_request_diff(diff)
    }

    pub fn response_field_info(&mut self, op: edge::DiffKind, field: edge::Field<'a>) -> &mut Self {
        let mut diff = edge::DiffData::default();
        diff.diff_type = edge::DiffDataType::field;
        diff.kind = op;
        diff.set_field(field).unwrap();
        self.add_response_diff(diff)
    }

    pub fn request_field(&mut self, op: edge::DiffKind, key: &'a str, value: &'a str) -> &mut Self {
        let field = make_field(key, value);
        self.request_field_info(op, field)
    }

    pub fn response_field(&mut self, op: edge::DiffKind, key: &'a str, value: &'a str) -> &mut Self {
        let field = make_field(key, value);
        self.response_field_info(op, field)
    }

    pub fn request_header_info(&mut self, op: edge::DiffKind, headers: edge::Header<'a>) -> &mut Self {
        let mut diff = edge::DiffData::default();
        diff.diff_type = edge::DiffDataType::header;
        diff.kind = op;
        diff.set_header(headers).unwrap();
        self.add_request_diff(diff)
    }

    pub fn response_header_info(&mut self, op: edge::DiffKind, headers: edge::Header<'a>) -> &mut Self {
        let mut diff = edge::DiffData::default();
        diff.diff_type = edge::DiffDataType::header;
        diff.kind = op;
        diff.set_header(headers).unwrap();
        self.add_response_diff(diff)
    }

    pub fn request_header<T: Iterator<Item = (&'a str, &'a str)>>(&mut self,op: edge::DiffKind, iter: T) -> &mut Self {
        let headers = make_header(iter);
        self.request_header_info(op, headers)
    }

    pub fn response_header<T: Iterator<Item = (&'a str, &'a str)>>(&mut self,op: edge::DiffKind, iter: T) -> &mut Self {
        let headers = make_header(iter);
        self.response_header_info(op, headers)
    }

    pub fn request_body_info(&mut self, body: edge::Body<'a>) -> &mut Self {
        let mut diff = edge::DiffData::default();
        diff.diff_type = edge::DiffDataType::body;
        diff.set_body(body).unwrap();
        self.add_request_diff(diff)
    }

    pub fn response_body_info(&mut self, body: edge::Body<'a>) -> &mut Self {
        let mut diff = edge::DiffData::default();
        diff.diff_type = edge::DiffDataType::body;
        diff.set_body(body).unwrap();
        self.add_response_diff(diff)
    }

    pub fn request_body(&mut self, data: &'a [u8]) -> &mut Self {
        let body = make_body(data);
        self.request_body_info(body)
    }

    pub fn response_body(&mut self, data: &'a [u8]) -> &mut Self {
        let body = make_body(data);
        self.response_body_info(body)
    }

    pub fn status(&mut self, status: u16) -> &mut Self {
        let mut diff = edge::DiffData::default();
        diff.diff_type = edge::DiffDataType::status;
        diff.set_status(status).unwrap();
        self.add_response_diff(diff)
    }

    pub fn apply_request(&self) -> Result<(), Error> {
        if self.request_changes.len == 0 {
            return Ok(()); // No changes to apply
        }
        let encoded = self.request_changes.encode_to_vec()?;
        if encoded.len() > u32::MAX as usize {
            return Err(Error::NotEnoughBuffer);
        }
        let size = encoded.len() as u32;
        let pointer = encoded.as_ptr() as *const u8;
        let result = unsafe {
            change_request_info(pointer, size)
        };
        if result == 0 {
            Err(Error::ApplyChangeFailed(true))
        } else {
            Ok(())
        }
    }

    pub fn apply_response(&self) -> Result<(), Error> {
        if self.response_changes.len == 0 {
            return Ok(()); // No changes to apply
        }
        let encoded = self.response_changes.encode_to_vec()?;
        if encoded.len() > u32::MAX as usize {
            return Err(Error::NotEnoughBuffer);
        }
        let size = encoded.len() as u32;
        let pointer = encoded.as_ptr() as *const u8;
        let result = unsafe {
            change_response_info(pointer, size)
        };
        if result == 0 {
            Err(Error::ApplyChangeFailed(false))
        } else {
            Ok(())
        }
    }

    pub fn apply(&self) -> Result<(), Error> {
        self.apply_request()?;
        self.apply_response()
    }


    // useful methods
    pub fn location(&mut self, location: &'a str) -> &mut Self {
        self.response_field(edge::DiffKind::replace, "Location", location)
    }
}

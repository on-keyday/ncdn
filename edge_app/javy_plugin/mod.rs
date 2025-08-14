
use crate::api;

use javy_plugin_api::{import_namespace, javy::quickjs::prelude::Func,javy::quickjs::Object,javy::quickjs::Null, Config};
use serde_json::json;
use std::{cell::RefCell, rc::Rc};

// Set your plugin's import namespace.
import_namespace!("ncdn_js");

struct NCDNContext {
    request: Option<serde_json::Value>,
    response: Option<serde_json::Value>,
    change_set :api::ChangeSet<'static>,
}

const BUF_SIZE: usize = 2048;

fn request_to_json(req: &api::Request) -> serde_json::Value {
    let headers = req.headers();
    let headers_map: serde_json::Map<String, serde_json::Value> = headers
        .map(|(k, v)| (k.to_string(), v.to_string().into()))
        .collect();
    json!({
        "method": req.method(),
        "path": req.path(),
        "headers": headers_map,
        "protocol": req.protocol(),
        "host": req.host(),
        "remote_addr": {
            "ip": req.remote_addr().0.to_string(),
            "port": req.remote_addr().1
        },
        "is_tls": req.is_tls(),
    })
}

fn response_to_json(resp: &api::Response) -> serde_json::Value {
    let headers = resp.headers();
    let headers_map: serde_json::Map<String, serde_json::Value> = headers
        .map(|(k, v)| (k.to_string(), v.to_string().into()))
        .collect();
    json!({
        "status": resp.status(),
        "headers": headers_map,
    })
}

#[export_name = "initialize_runtime"]
pub extern "C" fn initialize_runtime() {
    let config = Config::default();
    javy_plugin_api::initialize_runtime(config, |runtime| {
        runtime.context().with(|ctx| {
            let context = Rc::new(RefCell::new( NCDNContext {
                request: None,
                response: None,
                change_set: api::ChangeSet::new(),
            }));
            let ctx1 = context.clone();
            let jsctx = ctx.clone();
            ctx.globals().set("get_request",Func::new(move||{
                if let Some(s) = &ctx1.borrow().request {
                    jsctx.json_parse(s.to_string())
                } else {
                    let mut buffer = vec![0u8; BUF_SIZE];
                    match api::get_request(&mut buffer) {
                        Ok(request) => {
                            let req_json = request_to_json(&request);
                            let serialized = req_json.to_string();
                            ctx1.borrow_mut().request = Some(req_json);
                            jsctx.json_parse(serialized)
                        }
                        Err(e) => {
                            ctx1.borrow_mut().request = Some(json!({
                                "error": e.to_string(),
                            }));
                            jsctx.json_parse(json!({"error": e.to_string()}).to_string())
                        }
                    }
                }
            })).expect("Failed to set context");
            let ctx2 = context.clone();
            let jsctx2 = ctx.clone();
            ctx.globals().set("get_response", Func::new(move || {
                if let Some(s) = &ctx2.borrow().response {
                    jsctx2.json_parse(s.to_string())
                } else {
                    let mut buffer = vec![0u8; BUF_SIZE];
                    let response = api::get_response(&mut buffer);
                    match response {
                        Ok(resp) => {
                            let resp_json = response_to_json(&resp);
                            let serialized = resp_json.to_string();
                            ctx2.borrow_mut().response = Some(resp_json);
                            jsctx2.json_parse(serialized)  
                        }
                        Err(e) => {
                            ctx2.borrow_mut().response = Some(json!({
                                "error": e.to_string(),
                            }));
                            jsctx2.json_parse(json!({"error": e.to_string()}).to_string())
                        }
                    }
                }
            })).expect("Failed to set context");
            let ctx3 = context.clone();
            let jsctx3 = ctx.clone();
            ctx.globals().set("", Func::new(move || {
        
            }));
        });
        runtime
    })
    .unwrap();
}

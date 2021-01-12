use pool::blocking::{Connect, Pool};

use anyhow::{anyhow, Result};
use hyper::uri::RequestUri;
use hyper::server::{Request, Response, Server};
use hyper::status::StatusCode;
use hyper::Url;
use once_cell::sync::Lazy;
use serde::{Deserialize, Serialize};

use std::convert::TryFrom;
use std::io::{self, Read, Write};
use std::net::TcpStream;
use std::thread;

const HASHER_SERVICE_ADDR: &'static str = "127.0.0.1:34567";
const HANDLER_THREADS: usize = 10000;

struct Connection(TcpStream);

impl Connect for Connection {
    type Err = io::Error;

    fn connect() -> Result<Self, Self::Err> {
        Ok(Connection(TcpStream::connect(HASHER_SERVICE_ADDR)?))
    }
}

// A pool of connections to hasher service
static POOL: Lazy<Pool<Connection>> = Lazy::new(|| Pool::new());

#[derive(Debug, Deserialize)]
pub struct Input {
    pub message: String,
}

#[derive(Debug, Serialize)]
pub struct Output {
    pub message: String,
    pub hash: Option<String>, // hex-encoded
}

fn hash_value(value: &[u8]) -> Result<[u8; 32]> {
    let len = u16::try_from(value.len())
        .map_err(|_| anyhow!("Message length exceeds {} bytes", u16::MAX))?;

    let mut conn = POOL.get_connection()?;
    conn.0.write_all(&len.to_le_bytes())?;
    conn.0.write_all(value)?;

    let mut buf = [0u8; 32];
    conn.0.read_exact(&mut buf)?;
    Ok(buf)
}

fn digest_handler(req: Request) -> Result<Vec<u8>> {
    let uri = match req.uri {
        RequestUri::AbsolutePath(ref s) => Url::parse(&format!("http://example.com{}", s))?,
        RequestUri::AbsoluteUri(ref uri) => uri.to_owned(),
        _ => return Err(anyhow!("unsupported request URI")),
    };
    let input: Input = serde_json::from_reader(req)?;
    let hash = match uri.path() {
        "/hash" => Some(hex::encode(hash_value(input.message.as_bytes())?)),
        _ => None,
    };
    let output = Output {
        hash,
        message: input.message,
    };
    Ok(serde_json::to_vec(&output)?)
}

fn handle_request(req: Request, mut res: Response) {
    match digest_handler(req) {
        Ok(response) => res.send(&response).unwrap(),
        Err(e) => {
            *res.status_mut() = StatusCode::BadRequest;
            res.send(e.to_string().as_bytes()).unwrap();
        }
    }
}

fn main_loop() -> hyper::Result<()> {
    let incoming = Server::http("127.0.0.1:8080")?.handle_threads(handle_request, HANDLER_THREADS)?;
    println!("Echo service listening on: {}", incoming.socket);

    loop {
        thread::park();
    }
}

fn main() {
    main_loop().unwrap();
}

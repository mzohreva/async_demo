use pool::asynchronous::{Connect, Pool};

use anyhow::{anyhow, Result};
use bytes::Buf;
use http::{Request, Response, StatusCode};
use hyper::{rt, server::conn::Http, service::service_fn, Body};
use once_cell::sync::Lazy;
use serde::{Deserialize, Serialize};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream};
use tokio::runtime;

use std::convert::{Infallible, TryFrom};
use std::future::Future;
use std::io;
use std::pin::Pin;
use std::result::Result as StdResult;

const HASHER_SERVICE_ADDR: &'static str = "127.0.0.1:34567";

struct Connection(TcpStream);

impl Connect for Connection {
    type Err = io::Error;

    fn connect() -> Pin<Box<dyn Future<Output = Result<Self, Self::Err>> + Send>> {
        Box::pin(async move { Ok(Connection(TcpStream::connect(HASHER_SERVICE_ADDR).await?)) })
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

async fn hash_value(value: &[u8]) -> Result<[u8; 32]> {
    let len = u16::try_from(value.len())
        .map_err(|_| anyhow!("Message length exceeds {} bytes", u16::MAX))?;

    let mut conn = POOL.get_connection().await?;
    conn.0.write_u16_le(len).await?;
    conn.0.write_all(value).await?;

    let mut buf = [0u8; 32];
    conn.0.read_exact(&mut buf).await?;
    Ok(buf)
}

async fn digest_handler(req: Request<Body>) -> Result<Response<Body>> {
    let uri = req.uri().to_owned();
    let body = hyper::body::aggregate(req.into_body()).await?;
    let input: Input = serde_json::from_reader(body.reader())?;
    let hash = match uri.path() {
        "/hash" => Some(hex::encode(hash_value(input.message.as_bytes()).await?)),
        _ => None,
    };
    let output = Output {
        hash,
        message: input.message,
    };
    Ok(Response::new(Body::from(serde_json::to_vec(&output)?)))
}

async fn handle_request(req: Request<Body>) -> StdResult<Response<Body>, Infallible> {
    match digest_handler(req).await {
        Ok(response) => Ok(response),
        Err(e) => {
            let mut response = Response::new(Body::from(e.to_string()));
            *response.status_mut() = StatusCode::BAD_REQUEST;
            Ok(response)
        }
    }
}

async fn main_loop() -> Result<()> {
    let listener = TcpListener::bind("127.0.0.1:8080").await?;
    println!("Echo service listening on: {}", listener.local_addr()?);

    loop {
        let (socket, addr) = listener.accept().await?;
        tokio::spawn(async move {
            let http = Http::new().with_executor(Executor);
            let res = http
                .serve_connection(socket, service_fn(handle_request))
                .await;

            match res {
                Err(e) => println!("Error handling request from client {}: {}", addr, e),
                Ok(()) => {}
            }
        });
    }
}

fn main() {
    let rt = runtime::Builder::new_multi_thread()
        .enable_all()
        .worker_threads(4)
        .build()
        .expect("failed to initialize tokio runtime");

    rt.block_on(main_loop()).unwrap();
}

#[derive(Copy, Clone)]
pub struct Executor;

impl<F> rt::Executor<F> for Executor
where
    F: Future + Send + 'static,
    F::Output: Send + 'static,
{
    fn execute(&self, fut: F) {
        tokio::spawn(fut);
    }
}

/// async version uses crossbeam channels
pub mod asynchronous;
/// blocking version uses std::sync::Mutex
pub mod blocking;

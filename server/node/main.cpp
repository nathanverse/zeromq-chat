#include <unordered_map>
#include <utility>
#include <websocketpp/config/asio_no_tls.hpp>
#include <websocketpp/server.hpp>

#include <chrono>
#include <iostream>
#include <map>
#include <string>

typedef websocketpp::server<websocketpp::config::asio> server;

using websocketpp::lib::bind;
using websocketpp::lib::placeholders::_1;
using websocketpp::lib::placeholders::_2;

typedef server::message_ptr message_ptr;

class ConnectionManager {
public:
  void run(uint16_t port) {
    try {
      server_.set_access_channels(websocketpp::log::alevel::all);
      server_.clear_access_channels(websocketpp::log::alevel::frame_payload);

      server_.init_asio();

      server_.set_validate_handler(
          bind(&ConnectionManager::on_validate, this, ::_1));
      server_.set_http_handler(bind(&ConnectionManager::on_http, this, ::_1));
      server_.set_open_handler(bind(&ConnectionManager::on_open, this, ::_1));
      server_.set_close_handler(bind(&ConnectionManager::on_close, this, ::_1));
      server_.set_fail_handler(bind(&ConnectionManager::on_close, this, ::_1));
      server_.set_pong_handler(
          bind(&ConnectionManager::on_pong, this, ::_1, ::_2));
      server_.set_message_handler(
          bind(&ConnectionManager::on_message, this, ::_1, ::_2));

      start_ping_loop();

      server_.listen(port);
      server_.start_accept();
      server_.run();
    } catch (websocketpp::exception const &e) {
      std::cout << e.what() << std::endl;
    } catch (...) {
      std::cout << "other exception" << std::endl;
    }
  }

private:
  using clock = std::chrono::steady_clock;

  static constexpr long kPingIntervalMs = 25000;
  static constexpr long kPongTimeoutMs = 60000;

  enum class ParseErr { MissingEqual, MultipleEqual, EmptyKey, EmptyValue };
  // std::pair<std::string, std::string> & parseParam(std::string param) {
  //   if (param.empty()){
  //     return;
  //   }
  //   std::string key, value;
  //   int equalIndex = 0;
  //   for (size_t i = 0; i < param.size() ; i++) {
  //     if (equalIndex != 0) {
  //       value += param[i];  
  //     } else {
  //       key += param[i];
  //     }
  //   } 
  // }
  // std::unordered_map<std::string, std::string> & parseQuery(std::string query){
  //   std::string element;
  //   for (size_t i = 0; i < query.size() ; i++) {
  //     if query[i] == '&' {
  //       parseParam()  
  //     }
  //     element += query[i];
      
  //   }
  // }

  bool on_validate(websocketpp::connection_hdl hdl) {
    websocketpp::lib::error_code ec;
    server::connection_ptr con = server_.get_con_from_hdl(hdl, ec);
    if (ec) {
      return false;
    }

    auto uri = con -> get_uri();

    if (!uri && uri->get_resource() != "/ws") {
      con->set_status(websocketpp::http::status_code::not_found);
      return false;
    }

    auto query = uri -> get_query();

    
    return true;
  }

  void on_http(websocketpp::connection_hdl hdl) {
    websocketpp::lib::error_code ec;
    server::connection_ptr con = server_.get_con_from_hdl(hdl, ec);
    if (ec) {
      return;
    }
    if (con->get_resource() == "/healthz") {
      con->set_status(websocketpp::http::status_code::ok);
      con->set_body("ok");
      return;
    }
    con->set_status(websocketpp::http::status_code::not_found);
    con->set_body("not found");
  }

  void on_open(websocketpp::connection_hdl hdl) {
    connections_[hdl] = clock::now();
  }

  void on_close(websocketpp::connection_hdl hdl) { connections_.erase(hdl); }

  bool on_pong(websocketpp::connection_hdl hdl, std::string const &) {
    auto it = connections_.find(hdl);
    if (it != connections_.end()) {
      it->second = clock::now();
    }
    return true;
  }

  void on_message(websocketpp::connection_hdl hdl, message_ptr msg) {
    std::cout << "on_message called with hdl: " << hdl.lock().get()
              << " and message: " << msg->get_payload() << std::endl;

    if (msg->get_payload() == "stop-listening") {
      server_.stop_listening();
      return;
    }

    try {
      server_.send(hdl, msg->get_payload(), msg->get_opcode());
    } catch (websocketpp::exception const &e) {
      std::cout << "Echo failed because: "
                << "(" << e.what() << ")" << std::endl;
    }
  }

  void start_ping_loop() {
    server_.set_timer(kPingIntervalMs,
                      bind(&ConnectionManager::on_ping_timer, this, ::_1));
  }

  void on_ping_timer(websocketpp::lib::error_code const &ec) {
    if (ec) {
      return;
    }

    auto now = clock::now();
    for (auto it = connections_.begin(); it != connections_.end();) {
      auto age = std::chrono::duration_cast<std::chrono::milliseconds>(
          now - it->second);
      if (age.count() > kPongTimeoutMs) {
        websocketpp::lib::error_code close_ec;
        server_.close(it->first, websocketpp::close::status::going_away,
                      "pong timeout", close_ec);
        it = connections_.erase(it);
        continue;
      }

      websocketpp::lib::error_code ping_ec;
      server_.ping(it->first, "", ping_ec);
      ++it;
    }

    start_ping_loop();
  }

  server server_;
  std::map<websocketpp::connection_hdl, clock::time_point,
           std::owner_less<websocketpp::connection_hdl>>
      connections_;
};

int main() {
  ConnectionManager manager;
  manager.run(8080);
}

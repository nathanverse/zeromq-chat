vcpkg_from_github(
    OUT_SOURCE_PATH SOURCE_PATH
    REPO zaphoyd/websocketpp
    REF 0.8.2
    SHA512 b2afc63edb69ce81a3a6c06b3d857b3e8820f0e22300ac32bb20ab30ff07bd58bd5ada3e526ed8ab52de934e0e3a26cad2118b0e68ecf3e5e9e8d7101348fd06
)

file(INSTALL "${SOURCE_PATH}/websocketpp"
     DESTINATION "${CURRENT_PACKAGES_DIR}/include")

file(INSTALL "${SOURCE_PATH}/COPYING"
     DESTINATION "${CURRENT_PACKAGES_DIR}/share/websocketpp"
     RENAME copyright)

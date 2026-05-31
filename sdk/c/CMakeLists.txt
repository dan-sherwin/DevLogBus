cmake_minimum_required(VERSION 3.16)

project(devlogbus_c VERSION 1.3.1 LANGUAGES C)

include(CTest)

find_package(CURL REQUIRED)

add_library(devlogbus_c STATIC src/devlogbus.c)
add_library(devlogbus::c ALIAS devlogbus_c)

target_include_directories(devlogbus_c
    PUBLIC
        $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
        $<INSTALL_INTERFACE:include>
)
target_link_libraries(devlogbus_c PUBLIC CURL::libcurl)
set_target_properties(devlogbus_c PROPERTIES
    C_STANDARD 99
    C_STANDARD_REQUIRED ON
)

add_executable(devlogbus_c_basic examples/basic.c)
target_link_libraries(devlogbus_c_basic PRIVATE devlogbus_c)

if(BUILD_TESTING)
    add_executable(devlogbus_c_tests tests/test_devlogbus.c)
    target_link_libraries(devlogbus_c_tests PRIVATE devlogbus_c)
    add_test(NAME devlogbus_c_tests COMMAND devlogbus_c_tests)
endif()

install(TARGETS devlogbus_c
    ARCHIVE DESTINATION lib
    LIBRARY DESTINATION lib
    RUNTIME DESTINATION bin
)
install(DIRECTORY include/ DESTINATION include)

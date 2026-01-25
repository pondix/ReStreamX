cmake_minimum_required(VERSION 3.16)
project(restreamx_plugin CXX)

set(CMAKE_CXX_STANDARD 17)

find_path(MYSQL_INCLUDE_DIR mysql/plugin.h)
if(NOT MYSQL_INCLUDE_DIR)
  message(FATAL_ERROR "mysql headers not found")
endif()

add_library(restreamx SHARED
  src/restreamx_plugin.cc
  src/restreamx_sysvars.cc
  src/restreamx_capture.cc
  src/restreamx_apply.cc
  src/restreamx_lease.cc
  src/restreamx_ipc.cc
)

target_include_directories(restreamx PRIVATE ${MYSQL_INCLUDE_DIR} include)
set_target_properties(restreamx PROPERTIES OUTPUT_NAME "restreamx" PREFIX "" SUFFIX ".so")

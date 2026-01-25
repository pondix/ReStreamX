#include <mysql/plugin.h>
#include <mysql/plugin_audit.h>
#include <mysql/plugin_ftparser.h>
#include <mysql/plugin_replication_observer.h>
#include <mysql/plugin_services.h>
#include <mysql/service_my_snprintf.h>
#include <mysql/service_thd_wait.h>
#include <mysql/psi/mysql_thread.h>

#include <string>

extern "C" {

static const char *restreamx_mode = "OFF";
static char restreamx_node_id[128] = "";
static char restreamx_lease_range_ids[512] = "";
static char restreamx_ipc_socket_path[256] = "/var/run/restreamx.sock";
static bool restreamx_lease_owner = false;
static unsigned long long restreamx_lease_epoch = 0;

static int check_mode(MYSQL_THD, SYS_VAR *var, void *save, struct st_mysql_value *value) {
  char buff[16];
  int length = 0;
  const char *val = value->val_str(value, buff, &length);
  if (!val) return 1;
  if (strcmp(val, "OFF") && strcmp(val, "OWNER") && strcmp(val, "REPLICA")) return 1;
  *((const char **)save) = val;
  return 0;
}

static void update_mode(MYSQL_THD, SYS_VAR *, void *var_ptr, const void *save) {
  const char *new_val = *(const char **)save;
  *(const char **)var_ptr = new_val;
}

static MYSQL_SYSVAR_STR(mode, restreamx_mode, PLUGIN_VAR_RQCMDARG, "ReStreamX mode", check_mode, update_mode, "OFF");
static MYSQL_SYSVAR_STR(node_id, restreamx_node_id, PLUGIN_VAR_RQCMDARG, "ReStreamX node id", nullptr, nullptr, "");
static MYSQL_SYSVAR_STR(lease_range_ids, restreamx_lease_range_ids, PLUGIN_VAR_RQCMDARG, "Range IDs", nullptr, nullptr, "");
static MYSQL_SYSVAR_STR(ipc_socket_path, restreamx_ipc_socket_path, PLUGIN_VAR_RQCMDARG, "IPC socket", nullptr, nullptr, "/var/run/restreamx.sock");
static MYSQL_SYSVAR_BOOL(lease_owner, restreamx_lease_owner, PLUGIN_VAR_READONLY, "Lease owner", nullptr, nullptr, 0);
static MYSQL_SYSVAR_ULONGLONG(lease_epoch, restreamx_lease_epoch, PLUGIN_VAR_READONLY, "Lease epoch", nullptr, nullptr, 0, 0, ULLONG_MAX, 0);

static SYS_VAR *restreamx_system_vars[] = {
  MYSQL_SYSVAR(mode),
  MYSQL_SYSVAR(node_id),
  MYSQL_SYSVAR(lease_range_ids),
  MYSQL_SYSVAR(ipc_socket_path),
  MYSQL_SYSVAR(lease_owner),
  MYSQL_SYSVAR(lease_epoch),
  nullptr
};

const char *restreamx_get_mode() { return restreamx_mode; }
const char *restreamx_get_node_id() { return restreamx_node_id; }
const char *restreamx_get_ranges() { return restreamx_lease_range_ids; }
const char *restreamx_get_socket_path() { return restreamx_ipc_socket_path; }
void restreamx_set_lease_owner(bool owner) { restreamx_lease_owner = owner; }
void restreamx_set_lease_epoch(unsigned long long epoch) { restreamx_lease_epoch = epoch; }

SYS_VAR **restreamx_sysvars() { return restreamx_system_vars; }
}

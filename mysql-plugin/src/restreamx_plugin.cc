#include <mysql/plugin.h>
#include <mysql/plugin_audit.h>
#include <string.h>

extern "C" {
const char *restreamx_get_mode();
const char *restreamx_get_node_id();
const char *restreamx_get_ranges();
const char *restreamx_get_socket_path();
SYS_VAR **restreamx_sysvars();
}

static const char *apply_user = "restreamx_apply";

static bool is_write_query(const char *query) {
  if (!query) return false;
  while (*query == ' ' || *query == '\t' || *query == '\n') query++;
  return !strncasecmp(query, "insert", 6) || !strncasecmp(query, "update", 6) ||
         !strncasecmp(query, "delete", 6) || !strncasecmp(query, "replace", 7) ||
         !strncasecmp(query, "alter", 5) || !strncasecmp(query, "create", 6) ||
         !strncasecmp(query, "drop", 4);
}

static int restreamx_audit_notify(MYSQL_THD, mysql_event_class_t event_class, const void *event) {
  if (event_class != MYSQL_AUDIT_GENERAL_CLASS) return 0;
  const struct mysql_event_general *ev = (const struct mysql_event_general *)event;
  if (!ev || !ev->general_query) return 0;
  const char *mode = restreamx_get_mode();
  if (!mode) return 0;
  if (strcmp(mode, "REPLICA") == 0 && is_write_query(ev->general_query)) {
    if (ev->general_user && strcmp(ev->general_user, apply_user) == 0) {
      return 0;
    }
    return 1;
  }
  return 0;
}

static struct st_mysql_audit restreamx_audit_descriptor = {
  MYSQL_AUDIT_INTERFACE_VERSION,
  nullptr,
  nullptr,
  nullptr,
  nullptr,
  nullptr,
  restreamx_audit_notify,
  nullptr,
  nullptr,
  nullptr,
  nullptr,
};

static int restreamx_init(void *p) {
  (void)p;
  return 0;
}

static int restreamx_deinit(void *p) {
  (void)p;
  return 0;
}

static struct st_mysql_show_var restreamx_status_vars[] = {
  {"restreamx_mode", (char *)restreamx_get_mode(), SHOW_CHAR},
  {"restreamx_node_id", (char *)restreamx_get_node_id(), SHOW_CHAR},
  {"restreamx_ranges", (char *)restreamx_get_ranges(), SHOW_CHAR},
  {"restreamx_socket_path", (char *)restreamx_get_socket_path(), SHOW_CHAR},
  {nullptr, nullptr, SHOW_UNDEF}
};

mysql_declare_plugin(restreamx)
{
  MYSQL_AUDIT_PLUGIN,
  &restreamx_audit_descriptor,
  "restreamx",
  "ReStreamX",
  "Ledger-backed replication plugin",
  PLUGIN_LICENSE_GPL,
  restreamx_init,
  nullptr,
  restreamx_deinit,
  0x0100,
  restreamx_status_vars,
  restreamx_sysvars(),
  nullptr,
  0,
}
mysql_declare_plugin_end;

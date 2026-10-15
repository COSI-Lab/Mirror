/**
 * @file SyncScheduler.cpp
 * @author Cary Keesler
 * @brief
 */

// Header Being Defined
#include <mirror/sync_scheduler/SyncScheduler.hpp>

// Standard Library Includes
#include <cctype>
#include <cerrno>
#include <csignal>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <filesystem>
#include <format>
#include <fstream>
#include <ranges>
#include <stdexcept>
#include <string>
#include <string_view>
#include <thread>
#include <utility>

// Third Party Library Includes
#include <nlohmann/json_fwd.hpp>
// NOLINTNEXTLINE(misc-include-cleaner) Required to use a time variable in a log
#include <spdlog/fmt/bundled/chrono.h>
// NOLINTNEXTLINE(misc-include-cleaner) Required to print a range in a log
#include <spdlog/fmt/bundled/ranges.h>
#include <spdlog/spdlog.h>
#include <zmq.hpp>

// Project Includes
#include <mirror/sync_scheduler/ProjectCatalogue.hpp>
#include <mirror/sync_scheduler/SyncDetails.hpp>

namespace mirror::sync_scheduler
{
SyncScheduler::SyncScheduler()
try // Function try block my beloved
    : m_ProjectCatalogue(
          SyncScheduler::generate_project_catalogue(
              SyncScheduler::load_json_config("config/mirrors.json")
                  .value("mirrors", nlohmann::json())
          )
      ),
      m_Schedule(m_ProjectCatalogue),
      m_DryRun(false)
{
    if (const auto* dryRunPtr = std::getenv("DRY_RUN"))
    {
        std::string dryRun(dryRunPtr);
        std::transform(
            std::begin(dryRun),
            std::end(dryRun),
            std::begin(dryRun),
            [](auto c) { return std::toupper(c); }
        );

        if (dryRun == "TRUE")
        {
            m_DryRun = true;
        }
    }
    else
    {
        spdlog::warn(
            "Key \"DRY_RUN\" not set in the environment. Defaulting to false."
        );
        m_DryRun = false;
    }

    if (m_DryRun)
    {
        spdlog::info("Dry run enabled. No syncing will occur.");
    }
}
catch (std::runtime_error& re)
{
    spdlog::error(re.what());
    throw re;
}

auto SyncScheduler::load_json_config(const std::filesystem::path& file)
    -> nlohmann::json
{
    std::ifstream mirrorsConfigFile(file);

    if (!mirrorsConfigFile.good())
    {
        std::string errorMessage(BUFSIZ, '\0');

        throw std::runtime_error(
            std::format(
                "Failed to load config file {}! OS Error: {}",
                file.filename().string(),
                // NOLINTNEXTLINE(*-include-cleaner)
                ::strerror_r(errno, errorMessage.data(), errorMessage.size())
            )
        );
    }

    return nlohmann::json::parse(mirrorsConfigFile);
}

auto SyncScheduler::generate_project_catalogue(const nlohmann::json& mirrors)
    -> ProjectCatalogue
{
    ProjectCatalogue catalogue;

    if (mirrors.empty())
    {
        throw std::runtime_error("No mirrors found in config!");
    }

    for (const auto& [name, mirrorDetails] : mirrors.items())
    {
        try
        {
            catalogue.emplace(std::make_pair(name, SyncDetails(mirrorDetails)));
            spdlog::trace("Catalogued project {}", name);
        }
        catch (static_project_exception& spe)
        {
            spdlog::trace("{}: {}", name, spe.what());
        }
        catch (std::runtime_error& re)
        {
            spdlog::error("{}: {}", name, re.what());
        }
        catch (std::exception& e)
        {
            spdlog::error("{}: {}", name, e.what());
        }
    }

    return catalogue;
}

auto SyncScheduler::start_sync(const std::string& projectName) -> bool
{
    if (m_DryRun)
    {
        spdlog::info("Starting dry run sync for {}", projectName);
        return true;
    }

    spdlog::debug("Attempting sync for {}", projectName);

    const auto& syncDetails     = m_ProjectCatalogue.at(projectName);
    bool        startSuccessful = false;

    for (const auto& [idx, command] :
         syncDetails.get_commands() | std::views::enumerate)
    {
        const auto passwordFile = syncDetails.get_password_file();

        startSuccessful = m_JobManager.start_job(
            std::format(
                "{}{}",
                projectName,
                (idx != 0 ? "_part_" + std::to_string(idx) : "")
            ),
            command,
            (passwordFile.has_value() ? passwordFile.value()
                                      : std::filesystem::path {})
        );
    }

    if (startSuccessful)
    {
        spdlog::info("Successfully started sync for {}", projectName);
    }
    else
    {
        spdlog::warn("Failed to start sync for {}", projectName);
    }

    return startSuccessful;
}

auto SyncScheduler::manual_sync_loop() -> void
{
    using namespace std::string_view_literals;

    zmq::context_t socketContext {};
    zmq::socket_t  socket { socketContext, zmq::socket_type::rep };

    const std::string MANUAL_SYNC_PORT = []() -> std::string
    {
        // NOLINTNEXTLINE(*-mt-unsafe)
        const auto* manual_sync_env = std::getenv("MANUAL_SYNC_PORT");

        return (manual_sync_env != nullptr ? manual_sync_env : "9281");
    }();

    socket.bind(std::format("tcp://*:{}", MANUAL_SYNC_PORT));

    zmq::message_t syncRequest;

    while (true)
    {
        [[maybe_unused]]
        auto result
            = socket.recv(syncRequest, zmq::recv_flags::none);
        const std::string requestedProjectName = syncRequest.to_string();

        spdlog::info("Manual sync requested for {}", requestedProjectName);

        if (m_ProjectCatalogue.contains(requestedProjectName))
        {
            if (SyncScheduler::start_sync(requestedProjectName))
            {
                socket.send(
                    zmq::message_t(
                        std::format(
                            "SUCCESS: started sync for {}",
                            requestedProjectName
                        )
                    ),
                    zmq::send_flags::none
                );
                continue;
            }

            spdlog::error("Manual sync for {} failed", requestedProjectName);
            socket.send(
                zmq::message_t(
                    std::format(
                        "FAILURE: Failed to start sync for {}",
                        requestedProjectName
                    )
                ),
                zmq::send_flags::none
            );
            continue;
        }
        else if (requestedProjectName == "all_projects")
        {
            bool allSyncsStarted = true;

            for (const auto& [projectName, _] : m_ProjectCatalogue)
            {
                const bool syncStarted = SyncScheduler::start_sync(projectName);
                allSyncsStarted        = allSyncsStarted && syncStarted;

                if (!syncStarted)
                {
                    spdlog::error(
                        "Failed to start manual sync for {}",
                        projectName
                    );
                }
            }

            if (allSyncsStarted)
            {
                socket.send(
                    zmq::message_t("SUCCESS: started syncing all projects"sv),
                    zmq::send_flags::none
                );
                continue;
            }

            socket.send(
                zmq::message_t(
                    "FAILURE: Failed to start sync for some projects"sv
                ),
                zmq::send_flags::none
            );
            continue;
        }

        spdlog::error("Project {} not found!", requestedProjectName);
        socket.send(
            zmq::message_t(
                std::format(
                    "FAILURE: Project {} not found!",
                    requestedProjectName
                )
            ),
            zmq::send_flags::none
        );
    }
}

auto SyncScheduler::run() -> void
{
    spdlog::trace("Starting manual sync thread");
    m_ManualSyncThread = std::jthread(&SyncScheduler::manual_sync_loop, this);
    spdlog::info("Manual sync thread started!");

    spdlog::trace("Entering the main running loop");
    while (true)
    {
        const auto [timeOfNextSync, projectsToSync]
            = m_Schedule.get_next_sync_batch();

        spdlog::info(
            "Next sync scheduled for {:%m/%d @ %H:%M}. [{}]",
            timeOfNextSync,
            // NOLINTNEXTLINE(misc-include-cleaner)
            fmt::join(projectsToSync, ", ")
        );

        std::this_thread::sleep_until(timeOfNextSync);

        for (const auto& projectName : projectsToSync)
        {
            SyncScheduler::start_sync(projectName);
        }
    }

    spdlog::error("Leaving the main running loop! Program ending");
}
} // namespace mirror::sync_scheduler
